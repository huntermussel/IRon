#!/usr/bin/env python3
"""
deterministic_clone_finder.py

Deterministic similarity finder for codebases:
- Extracts blocks (heuristic: functions/classes + fallback sliding windows)
- Normalizes code (comments/whitespace, literals, identifiers)
- Builds fingerprints via k-gram hashing + winnowing
- Compares blocks with Jaccard similarity of fingerprints
- Outputs clusters of similar blocks (JSON) and a short text report

Usage:
  python3 deterministic_clone_finder.py --root . --min-jaccard 0.55 --out clones.json
  python3 deterministic_clone_finder.py --root . --langs ts,js,go,rb --windows 0
  python3 deterministic_clone_finder.py --root . --mode windows --win-lines 20 --min-jaccard 0.65

Notes:
- Heuristic parser (no AST). Works best for Go/JS/TS/Python/Ruby-ish styles.
- Deterministic: no embeddings, no randomness.
"""

# ToolSchema: {"description": "Find code clones/similarity in a codebase using deterministic winnowing.", "parameters": {"type": "object", "properties": {"root": {"type": "string", "description": "Root directory to scan"}, "langs": {"type": "string", "description": "Comma-separated extensions (go,py,js)"}, "mode": {"type": "string", "enum": ["blocks", "windows", "both"]}, "min-jaccard": {"type": "number", "description": "Similarity threshold (0.0-1.0)"}}, "required": ["root"]}}

from __future__ import annotations

import argparse
import dataclasses
import hashlib
import json
import os
import re
from collections import defaultdict, deque
from pathlib import Path
from typing import Dict, Iterable, List, Optional, Set, Tuple

# ----------------------------
# Config / file filtering
# ----------------------------

DEFAULT_EXCLUDE_DIRS = {
    ".git",
    ".hg",
    ".svn",
    "node_modules",
    "vendor",
    "dist",
    "build",
    "target",
    ".next",
    ".nuxt",
    "__pycache__",
    ".venv",
    "venv",
    "coverage",
    ".idea",
    ".vscode",
}

EXT_LANG = {
    ".go": "go",
    ".js": "js",
    ".ts": "ts",
    ".jsx": "jsx",
    ".tsx": "tsx",
    ".py": "py",
    ".rb": "rb",
    ".java": "java",
    ".kt": "kt",
    ".rs": "rs",
    ".php": "php",
}

# ----------------------------
# Data structures
# ----------------------------


@dataclasses.dataclass(frozen=True)
class BlockRef:
    file: str
    start_line: int
    end_line: int
    kind: str  # function|class|window
    name: str  # best-effort
    lang: str


@dataclasses.dataclass
class Block:
    ref: BlockRef
    text: str
    norm_tokens: List[str]
    fingerprints: Set[int]


# ----------------------------
# Normalization / tokenization
# ----------------------------

RE_WS = re.compile(r"\s+")
RE_STR = re.compile(
    r"""(?s)(\"\"\".*?\"\"\"|'''.*?'''|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')"""
)
RE_NUM = re.compile(r"\b\d+(?:\.\d+)?\b")
RE_HEX = re.compile(r"\b0x[0-9a-fA-F]+\b")

# crude comment patterns for multiple langs
RE_LINE_COMMENT = re.compile(r"(?m)^\s*(//|#).*$")
RE_INLINE_LINE_COMMENT = re.compile(r"(?m)(?<!:)//.*$")  # tries not to kill URLs
RE_BLOCK_COMMENT = re.compile(r"(?s)/\*.*?\*/")

# identifiers: keep keywords-ish, normalize most names to ID
RE_IDENT = re.compile(r"\b[A-Za-z_][A-Za-z0-9_]*\b")

# A small keyword set to preserve structure
KEYWORDS = set(
    """
if else for while switch case break continue return try catch finally throw
func function def class struct interface type package import from export const let var
public private protected static async await yield new this super
do in of nil null true false
map filter reduce foreach
""".split()
)


def strip_comments(code: str, lang: str) -> str:
    # keep it simple and deterministic
    s = code
    # block comments common in C-like
    s = RE_BLOCK_COMMENT.sub(" ", s)
    # line comments
    if lang in {"py", "rb"}:
        s = RE_LINE_COMMENT.sub(" ", s)
    else:
        s = RE_INLINE_LINE_COMMENT.sub(" ", s)
    return s


def normalize_code(code: str, lang: str) -> str:
    s = strip_comments(code, lang)
    s = RE_STR.sub(" STR ", s)
    s = RE_HEX.sub(" NUM ", s)
    s = RE_NUM.sub(" NUM ", s)
    # collapse whitespace
    s = RE_WS.sub(" ", s).strip()
    return s


def tokenize(code_norm: str) -> List[str]:
    """
    Tokenize by splitting into identifiers/operators/punct.
    Deterministic, low-tech.
    """
    tokens: List[str] = []
    i = 0
    n = len(code_norm)
    while i < n:
        ch = code_norm[i]
        if ch.isspace():
            i += 1
            continue
        # identifier
        if ch.isalpha() or ch == "_":
            j = i + 1
            while j < n and (code_norm[j].isalnum() or code_norm[j] == "_"):
                j += 1
            tok = code_norm[i:j]
            if tok in KEYWORDS:
                tokens.append(tok)
            elif tok in {"STR", "NUM"}:
                tokens.append(tok)
            else:
                tokens.append("ID")
            i = j
            continue
        # numbers already normalized to NUM, but keep any leftover digits
        if ch.isdigit():
            j = i + 1
            while j < n and (code_norm[j].isdigit() or code_norm[j] == "."):
                j += 1
            tokens.append("NUM")
            i = j
            continue
        # operators/punct (single char; enough for similarity)
        tokens.append(ch)
        i += 1
    # remove trivial tokens that create noise
    tokens = [t for t in tokens if t not in {";", ","}]  # keep braces/parens etc
    return tokens


# ----------------------------
# Fingerprinting (k-grams + winnowing)
# ----------------------------


def stable_hash_int(s: str) -> int:
    # stable across runs/platforms
    h = hashlib.blake2b(s.encode("utf-8"), digest_size=8).digest()
    return int.from_bytes(h, "big", signed=False)


def kgram_hashes(tokens: List[str], k: int) -> List[int]:
    if len(tokens) < k:
        return []
    out = []
    for i in range(len(tokens) - k + 1):
        gram = " ".join(tokens[i : i + k])
        out.append(stable_hash_int(gram))
    return out


def winnow(hashes: List[int], window: int) -> Set[int]:
    """
    Winnowing: choose minimum hash in each window (with rightmost tie-break).
    """
    if not hashes:
        return set()
    if window <= 1:
        return set(hashes)

    selected: Set[int] = set()
    dq = deque()  # (hash, idx)

    for i, hv in enumerate(hashes):
        # pop bigger hashes from the right
        while dq and dq[-1][0] >= hv:
            dq.pop()
        dq.append((hv, i))
        # drop left elements outside window
        while dq and dq[0][1] <= i - window:
            dq.popleft()
        # start selecting once we have a full window
        if i >= window - 1 and dq:
            selected.add(dq[0][0])
    return selected


def fingerprints_for_tokens(tokens: List[str], k: int, w: int) -> Set[int]:
    hs = kgram_hashes(tokens, k)
    return winnow(hs, w)


def jaccard(a: Set[int], b: Set[int]) -> float:
    if not a or not b:
        return 0.0
    inter = len(a & b)
    if inter == 0:
        return 0.0
    uni = len(a | b)
    return inter / uni if uni else 0.0


# ----------------------------
# Block extraction (heuristic)
# ----------------------------

RE_GO_FUNC = re.compile(
    r"^\s*func\s+(\([^\)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(", re.M
)
RE_GO_TYPE = re.compile(
    r"^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(struct|interface)\b", re.M
)

RE_JS_FUNC = re.compile(
    r"^\s*(export\s+)?(async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(", re.M
)
RE_JS_CLASS = re.compile(r"^\s*(export\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)\b", re.M)
RE_JS_ARROW = re.compile(
    r"^\s*(export\s+)?(const|let|var)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(async\s*)?\(",
    re.M,
)

RE_PY_DEF = re.compile(r"^\s*def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(", re.M)
RE_PY_CLASS = re.compile(r"^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)\b", re.M)

RE_RB_DEF = re.compile(r"^\s*def\s+([A-Za-z_][A-Za-z0-9_!?=]*)", re.M)
RE_RB_CLASS = re.compile(r"^\s*class\s+([A-Za-z_][A-Za-z0-9_:]*)", re.M)


def find_matching_blocks(code: str, lang: str) -> List[Tuple[int, int, str, str]]:
    """
    Returns list of (start_line, end_line, kind, name) line numbers are 1-based.
    Heuristic braces for C-like; indentation for Python/Ruby.
    """
    lines = code.splitlines()
    n = len(lines)

    def brace_block_from_line(start_idx: int) -> Optional[int]:
        # start_idx is 0-based
        brace = 0
        started = False
        for i in range(start_idx, n):
            ln = lines[i]
            # count braces
            for ch in ln:
                if ch == "{":
                    brace += 1
                    started = True
                elif ch == "}":
                    brace -= 1
            if started and brace <= 0:
                return i
        return None

    def indent_block_from_line(start_idx: int) -> int:
        # Python/Ruby-ish: block ends when indentation decreases (very heuristic)
        base = len(lines[start_idx]) - len(lines[start_idx].lstrip(" "))
        end = start_idx
        for i in range(start_idx + 1, n):
            ln = lines[i]
            if not ln.strip():
                end = i
                continue
            ind = len(ln) - len(ln.lstrip(" "))
            if ind <= base and not ln.lstrip().startswith(("#",)):
                break
            end = i
        return end

    matches: List[Tuple[int, int, str, str]] = []

    def add_matches(regex: re.Pattern, kind: str, name_group: int):
        for m in regex.finditer(code):
            start_line = code[: m.start()].count("\n") + 1
            start_idx = start_line - 1
            name = m.group(name_group) if name_group <= m.lastindex else "unknown"
            if lang in {"py", "rb"}:
                end_idx = indent_block_from_line(start_idx)
            else:
                end_idx = brace_block_from_line(start_idx)
                if end_idx is None:
                    continue
            matches.append((start_idx + 1, end_idx + 1, kind, name))

    if lang == "go":
        add_matches(RE_GO_FUNC, "function", 2)
        add_matches(RE_GO_TYPE, "type", 1)
    elif lang in {"js", "ts", "jsx", "tsx"}:
        add_matches(RE_JS_FUNC, "function", 3)
        add_matches(RE_JS_CLASS, "class", 2)
        add_matches(RE_JS_ARROW, "function", 3)
    elif lang == "py":
        add_matches(RE_PY_DEF, "function", 1)
        add_matches(RE_PY_CLASS, "class", 1)
    elif lang == "rb":
        add_matches(RE_RB_DEF, "function", 1)
        add_matches(RE_RB_CLASS, "class", 1)
    else:
        # fallback: no matches
        pass

    # dedupe overlaps: keep larger blocks first
    matches.sort(key=lambda x: (x[0], -(x[1] - x[0])))
    pruned: List[Tuple[int, int, str, str]] = []
    last_end = 0
    for s, e, k, nm in matches:
        if s >= last_end:
            pruned.append((s, e, k, nm))
            last_end = e
    return pruned


def make_windows(
    code: str, win_lines: int, step: int
) -> List[Tuple[int, int, str, str]]:
    lines = code.splitlines()
    n = len(lines)
    out = []
    if win_lines <= 0 or win_lines > n:
        return out
    for s in range(0, n - win_lines + 1, max(1, step)):
        e = s + win_lines
        out.append((s + 1, e, "window", f"window_{s + 1}_{e}"))
    return out


# ----------------------------
# Scanning / indexing
# ----------------------------


def iter_files(
    root: Path, langs: Set[str], max_file_kb: int, exclude_dirs: Set[str]
) -> Iterable[Path]:
    for dirpath, dirnames, filenames in os.walk(root):
        # prune excluded dirs
        dirnames[:] = [
            d for d in dirnames if d not in exclude_dirs and not d.startswith(".")
        ]
        for fn in filenames:
            p = Path(dirpath) / fn
            ext = p.suffix.lower()
            if ext not in EXT_LANG:
                continue
            lang = EXT_LANG[ext]
            if langs and lang not in langs:
                continue
            try:
                if p.stat().st_size > max_file_kb * 1024:
                    continue
            except OSError:
                continue
            yield p


def read_text(p: Path) -> str:
    try:
        return p.read_text(encoding="utf-8", errors="replace")
    except Exception:
        return ""


def build_blocks(
    root: Path,
    langs: Set[str],
    mode: str,
    win_lines: int,
    win_step: int,
    min_block_lines: int,
    max_file_kb: int,
    exclude_dirs: Set[str],
) -> List[Tuple[BlockRef, str]]:
    blocks: List[Tuple[BlockRef, str]] = []
    for fp in iter_files(root, langs, max_file_kb, exclude_dirs):
        code = read_text(fp)
        if not code.strip():
            continue
        ext_lang = EXT_LANG.get(fp.suffix.lower(), "unknown")

        file_rel = str(fp.relative_to(root))

        if mode in {"blocks", "both"}:
            found = find_matching_blocks(code, ext_lang)
            for s, e, kind, name in found:
                if e - s + 1 < min_block_lines:
                    continue
                text = "\n".join(code.splitlines()[s - 1 : e])
                blocks.append((BlockRef(file_rel, s, e, kind, name, ext_lang), text))

        if mode in {"windows", "both"}:
            wins = make_windows(code, win_lines, win_step)
            for s, e, kind, name in wins:
                text = "\n".join(code.splitlines()[s - 1 : e])
                blocks.append((BlockRef(file_rel, s, e, kind, name, ext_lang), text))

    return blocks


def block_to_fingerprinted(
    bref: BlockRef,
    text: str,
    k: int,
    w: int,
    min_tokens: int,
) -> Optional[Block]:
    norm = normalize_code(text, bref.lang)
    toks = tokenize(norm)
    if len(toks) < min_tokens:
        return None
    fps = fingerprints_for_tokens(toks, k, w)
    if not fps:
        return None
    return Block(ref=bref, text=text, norm_tokens=toks, fingerprints=fps)


# ----------------------------
# Candidate generation + clustering
# ----------------------------


def union_find(n: int):
    parent = list(range(n))
    rank = [0] * n

    def find(x: int) -> int:
        while parent[x] != x:
            parent[x] = parent[parent[x]]
            x = parent[x]
        return x

    def union(a: int, b: int):
        ra, rb = find(a), find(b)
        if ra == rb:
            return
        if rank[ra] < rank[rb]:
            parent[ra] = rb
        elif rank[ra] > rank[rb]:
            parent[rb] = ra
        else:
            parent[rb] = ra
            rank[ra] += 1

    return find, union, parent


def cluster_blocks(
    blocks: List[Block],
    min_jaccard: float,
    min_shared_fps: int,
    topk_per_block: int,
) -> Tuple[List[List[int]], Dict[Tuple[int, int], float]]:
    """
    Deterministic clustering:
    - Build inverted index: fingerprint -> block indices
    - For each block, count overlaps with candidates
    - Compute Jaccard for top candidates; union if threshold
    """
    inv: Dict[int, List[int]] = defaultdict(list)
    for i, b in enumerate(blocks):
        for fp in b.fingerprints:
            inv[fp].append(i)

    # overlap counting
    pair_scores: Dict[Tuple[int, int], float] = {}
    find, union, _parent = union_find(len(blocks))

    for i, b in enumerate(blocks):
        counts: Dict[int, int] = defaultdict(int)
        for fp in b.fingerprints:
            for j in inv.get(fp, []):
                if j == i:
                    continue
                counts[j] += 1

        # shortlist by shared fps (descending), deterministic tie-break by index
        cand = [(j, c) for j, c in counts.items() if c >= min_shared_fps]
        cand.sort(key=lambda x: (-x[1], x[0]))
        cand = cand[: max(0, topk_per_block)]

        for j, shared in cand:
            a, c = (i, j) if i < j else (j, i)
            if (a, c) in pair_scores:
                continue
            sim = jaccard(blocks[a].fingerprints, blocks[c].fingerprints)
            if sim >= min_jaccard:
                pair_scores[(a, c)] = sim
                union(a, c)

    # collect clusters
    buckets: Dict[int, List[int]] = defaultdict(list)
    for i in range(len(blocks)):
        buckets[find(i)].append(i)

    clusters = [sorted(v) for v in buckets.values() if len(v) >= 2]
    # stable ordering: bigger clusters first, then by first index
    clusters.sort(key=lambda cl: (-len(cl), cl[0]))
    return clusters, pair_scores


# ----------------------------
# Output
# ----------------------------


def bref_to_dict(b: BlockRef) -> dict:
    return {
        "file": b.file,
        "start_line": b.start_line,
        "end_line": b.end_line,
        "kind": b.kind,
        "name": b.name,
        "lang": b.lang,
    }


def render_text_report(
    blocks: List[Block],
    clusters: List[List[int]],
    pair_scores: Dict[Tuple[int, int], float],
    max_clusters: int,
) -> str:
    out = []
    out.append(f"Clusters found: {len(clusters)}")
    out.append("")
    for ci, cl in enumerate(clusters[:max_clusters], start=1):
        out.append(f"== Cluster {ci} (size={len(cl)}) ==")
        # show members
        for idx in cl:
            r = blocks[idx].ref
            out.append(
                f"- [{idx}] {r.lang} {r.kind} {r.name} :: {r.file}:{r.start_line}-{r.end_line}"
            )
        # show a few pair scores
        best_pairs = []
        for a in cl:
            for b in cl:
                if a >= b:
                    continue
                key = (a, b)
                if key in pair_scores:
                    best_pairs.append((pair_scores[key], a, b))
        best_pairs.sort(reverse=True)
        for sim, a, b in best_pairs[:5]:
            out.append(f"  sim={sim:.3f}  [{a}] <-> [{b}]")
        out.append("")
    return "\n".join(out)


# ----------------------------
# Main
# ----------------------------


def parse_args() -> argparse.Namespace:
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", default=".", help="Repository root")
    ap.add_argument(
        "--langs",
        default="",
        help="Comma-separated: go,js,ts,jsx,tsx,py,rb,java,kt,rs,php (empty=all)",
    )
    ap.add_argument(
        "--exclude-dirs",
        default=",".join(sorted(DEFAULT_EXCLUDE_DIRS)),
        help="Comma-separated dir names to skip",
    )
    ap.add_argument(
        "--max-file-kb", type=int, default=512, help="Skip files bigger than this"
    )
    ap.add_argument(
        "--mode",
        choices=["blocks", "windows", "both"],
        default="blocks",
        help="Extraction mode",
    )
    ap.add_argument(
        "--min-block-lines", type=int, default=12, help="Min lines for a parsed block"
    )
    ap.add_argument(
        "--win-lines", type=int, default=20, help="Window size in lines (windows/both)"
    )
    ap.add_argument(
        "--win-step", type=int, default=10, help="Window step in lines (windows/both)"
    )

    ap.add_argument("--k", type=int, default=8, help="k-gram size (tokens)")
    ap.add_argument(
        "--w", type=int, default=10, help="winnowing window size (k-gram hashes)"
    )
    ap.add_argument(
        "--min-tokens",
        type=int,
        default=80,
        help="Min normalized tokens per block/window",
    )

    ap.add_argument(
        "--min-jaccard", type=float, default=0.55, help="Similarity threshold"
    )
    ap.add_argument(
        "--min-shared-fps",
        type=int,
        default=12,
        help="Min shared fingerprints to consider pair",
    )
    ap.add_argument(
        "--topk", type=int, default=50, help="Max candidate comparisons per block"
    )

    ap.add_argument(
        "--max-clusters",
        type=int,
        default=50,
        help="Max clusters to show in text report",
    )
    ap.add_argument("--out", default="clones.json", help="Output JSON file path")
    ap.add_argument("--report", default="clones.txt", help="Output text report path")
    return ap.parse_args()


def main():
    args = parse_args()
    root = Path(args.root).resolve()

    langs = (
        set([s.strip() for s in args.langs.split(",") if s.strip()])
        if args.langs.strip()
        else set()
    )
    exclude_dirs = set([s.strip() for s in args.exclude_dirs.split(",") if s.strip()])

    raw_blocks = build_blocks(
        root=root,
        langs=langs,
        mode=args.mode,
        win_lines=args.win_lines,
        win_step=args.win_step,
        min_block_lines=args.min_block_lines,
        max_file_kb=args.max_file_kb,
        exclude_dirs=exclude_dirs,
    )

    blocks: List[Block] = []
    for bref, text in raw_blocks:
        b = block_to_fingerprinted(
            bref, text, k=args.k, w=args.w, min_tokens=args.min_tokens
        )
        if b is not None:
            blocks.append(b)

    clusters, pair_scores = cluster_blocks(
        blocks=blocks,
        min_jaccard=args.min_jaccard,
        min_shared_fps=args.min_shared_fps,
        topk_per_block=args.topk,
    )

    # Build JSON
    clusters_json = []
    for cl in clusters:
        members = []
        for idx in cl:
            members.append(
                {
                    "idx": idx,
                    "ref": bref_to_dict(blocks[idx].ref),
                    "fingerprints": len(blocks[idx].fingerprints),
                    "tokens": len(blocks[idx].norm_tokens),
                }
            )
        # include best pair similarity in the cluster (for quick ranking)
        best = 0.0
        for a in cl:
            for b in cl:
                if a >= b:
                    continue
                sim = pair_scores.get((a, b), 0.0)
                if sim > best:
                    best = sim
        clusters_json.append(
            {
                "size": len(cl),
                "best_pair_jaccard": best,
                "members": members,
            }
        )

    out_obj = {
        "root": str(root),
        "settings": {
            "mode": args.mode,
            "langs": sorted(langs) if langs else "all",
            "k": args.k,
            "w": args.w,
            "min_tokens": args.min_tokens,
            "min_jaccard": args.min_jaccard,
            "min_shared_fps": args.min_shared_fps,
            "topk": args.topk,
            "min_block_lines": args.min_block_lines,
            "win_lines": args.win_lines,
            "win_step": args.win_step,
            "max_file_kb": args.max_file_kb,
        },
        "stats": {
            "raw_blocks": len(raw_blocks),
            "indexed_blocks": len(blocks),
            "clusters": len(clusters),
            "pairs_kept": len(pair_scores),
        },
        "clusters": clusters_json,
    }

    Path(args.out).write_text(json.dumps(out_obj, indent=2), encoding="utf-8")
    report = render_text_report(blocks, clusters, pair_scores, args.max_clusters)
    Path(args.report).write_text(report, encoding="utf-8")

    print(f"Wrote: {args.out}")
    print(f"Wrote: {args.report}")
    print(
        f"Indexed blocks: {len(blocks)} | clusters: {len(clusters)} | pairs: {len(pair_scores)}"
    )


if __name__ == "__main__":
    main()
