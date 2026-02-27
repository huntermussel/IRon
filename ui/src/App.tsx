import React, { useState, useEffect, useRef } from "react";
import {
  Shield,
  LayoutDashboard,
  MessageSquare,
  Settings,
  Server,
  Bot,
  Brain,
  Check,
  Send,
} from "lucide-react";
import { marked } from "marked";

// --- Types ---
interface Config {
  provider: string;
  model: string;
  base_url?: string;
  api_key?: string;
  scripts_dir?: string;
}

interface Message {
  role: "user" | "bot";
  content: string;
}

interface Status {
  isOnline: boolean;
  time: string;
}

export default function App() {
  // --- State ---
  const [activeTab, setActiveTab] = useState<"dashboard" | "chat" | "settings">(
    "dashboard",
  );
  const [status, setStatus] = useState<Status>({ isOnline: false, time: "" });
  const [config, setConfig] = useState<Config>({
    provider: "ollama",
    model: "llama3.2",
    base_url: "",
    api_key: "",
    scripts_dir: "scripts",
  });

  // Chat State
  const [messages, setMessages] = useState<Message[]>([
    {
      role: "bot",
      content:
        "Hello! I'm IRon, your personal AI assistant. How can I help you today?",
    },
  ]);
  const [chatInput, setChatInput] = useState("");
  const [isWaiting, setIsWaiting] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Settings State
  const [localConfig, setLocalConfig] = useState<Config>(config);
  const [showSaveSuccess, setShowSaveSuccess] = useState(false);

  // --- Effects ---
  useEffect(() => {
    fetchStatus();
    loadSettings();
    const interval = setInterval(fetchStatus, 30000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, isWaiting]);

  useEffect(() => {
    setLocalConfig(config);
  }, [config]);

  // --- API Calls ---
  const fetchStatus = async () => {
    try {
      const res = await fetch("/api/status");
      if (res.ok) {
        const data = await res.json();
        setStatus({
          isOnline: true,
          time: new Date(data.time).toLocaleTimeString(),
        });
      } else {
        setStatus((s) => ({ ...s, isOnline: false }));
      }
    } catch (e) {
      setStatus((s) => ({ ...s, isOnline: false }));
    }
  };

  const loadSettings = async () => {
    try {
      const res = await fetch("/api/settings");
      if (res.ok) {
        const data = await res.json();
        setConfig({
          provider: data.provider || "ollama",
          model: data.model || "llama3.2",
          base_url: data.base_url || "",
          api_key: data.api_key || "",
          scripts_dir: data.scripts_dir || "scripts",
        });
      }
    } catch (e) {
      console.error("Failed to load settings", e);
    }
  };

  const saveSettings = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await fetch("/api/settings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(localConfig),
      });
      if (res.ok) {
        setConfig(localConfig);
        setShowSaveSuccess(true);
        setTimeout(() => setShowSaveSuccess(false), 3000);
      }
    } catch (e: any) {
      alert("Failed to save settings: " + e.message);
    }
  };

  const sendMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!chatInput.trim() || isWaiting) return;

    const userText = chatInput.trim();
    setMessages((prev) => [...prev, { role: "user", content: userText }]);
    setChatInput("");
    setIsWaiting(true);

    try {
      const res = await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message: userText }),
      });
      if (res.ok) {
        const data = await res.json();
        if (data.error) {
          setMessages((prev) => [
            ...prev,
            { role: "bot", content: "⚠️ Error: " + data.error },
          ]);
        } else {
          setMessages((prev) => [
            ...prev,
            { role: "bot", content: data.reply || "..." },
          ]);
        }
      } else {
        setMessages((prev) => [
          ...prev,
          { role: "bot", content: "⚠️ Failed to communicate with the server." },
        ]);
      }
    } catch (err) {
      setMessages((prev) => [
        ...prev,
        { role: "bot", content: "⚠️ Network error occurred." },
      ]);
    } finally {
      setIsWaiting(false);
    }
  };

  // --- Helpers ---
  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  const renderMarkdown = (text: string) => {
    return { __html: marked.parse(text) as string };
  };

  // --- Sub-components ---
  const renderDashboard = () => (
    <div className="flex-1 overflow-y-auto p-6 md:p-10">
      <h2 className="text-3xl font-bold mb-6">Dashboard</h2>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <div className="bg-gray-800 rounded-lg p-6 shadow border border-gray-700">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-gray-400 font-semibold">System Status</h3>
            <Server className="text-blue-500 w-5 h-5" />
          </div>
          <div className="text-2xl font-bold flex items-center">
            {status.isOnline ? (
              <>
                <span className="w-3 h-3 rounded-full bg-green-500 mr-3 shadow-[0_0_10px_#22c55e]"></span>{" "}
                Running
              </>
            ) : (
              <>
                <span className="w-3 h-3 rounded-full bg-red-500 mr-3 shadow-[0_0_10px_#ef4444]"></span>{" "}
                Offline
              </>
            )}
          </div>
          <p className="text-sm text-gray-500 mt-2">
            {status.time ? `Last checked: ${status.time}` : "Checking time..."}
          </p>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 shadow border border-gray-700">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-gray-400 font-semibold">Active Agent</h3>
            <Bot className="text-purple-500 w-5 h-5" />
          </div>
          <div className="text-2xl font-bold">
            {config.model || "Loading..."}
          </div>
          <p className="text-sm text-gray-500 mt-2">
            Provider: {config.provider || "Loading..."}
          </p>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 shadow border border-gray-700">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-gray-400 font-semibold">Session Context</h3>
            <Brain className="text-yellow-500 w-5 h-5" />
          </div>
          <div className="text-2xl font-bold">Active</div>
          <p className="text-sm text-gray-500 mt-2">Memory store operational</p>
        </div>
      </div>

      <div className="bg-gray-800 rounded-lg p-6 shadow border border-gray-700">
        <h3 className="text-xl font-bold mb-4">Features & Integrations</h3>
        <ul className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-gray-300">
          <li className="flex items-center">
            <Check className="text-green-500 mr-3 w-5 h-5" /> CLI & Terminal
            execution
          </li>
          <li className="flex items-center">
            <Check className="text-green-500 mr-3 w-5 h-5" /> Headless Browser
            Skills
          </li>
          <li className="flex items-center">
            <Check className="text-green-500 mr-3 w-5 h-5" /> Vector Memory
            Store
          </li>
          <li className="flex items-center">
            <Check className="text-green-500 mr-3 w-5 h-5" /> Extensible
            Middlewares
          </li>
          <li className="flex items-center">
            <Check className="text-green-500 mr-3 w-5 h-5" /> Telegram Bot
            Adapter
          </li>
          <li className="flex items-center">
            <Check className="text-green-500 mr-3 w-5 h-5" /> Slack Adapter
            (WIP)
          </li>
        </ul>
      </div>
    </div>
  );

  const renderChat = () => (
    <div className="flex-1 flex flex-col h-full overflow-hidden relative">
      <div className="flex-1 overflow-y-auto p-4 md:p-6 space-y-6">
        {messages.map((msg, idx) => (
          <div
            key={idx}
            className={`flex ${msg.role === "user" ? "justify-end" : ""}`}
          >
            {msg.role === "bot" && (
              <div className="w-8 h-8 rounded-full bg-gray-700 flex items-center justify-center mr-3 flex-shrink-0 mt-1 border border-gray-600">
                <Bot className="w-4 h-4 text-blue-400" />
              </div>
            )}

            <div
              className={`rounded-2xl px-5 py-3 max-w-[85%] shadow-sm ${
                msg.role === "user"
                  ? "bg-blue-600 rounded-tr-none text-white"
                  : "bg-gray-800 rounded-tl-none border border-gray-700 markdown-body"
              }`}
            >
              {msg.role === "user" ? (
                <p className="whitespace-pre-wrap">{msg.content}</p>
              ) : (
                <div dangerouslySetInnerHTML={renderMarkdown(msg.content)} />
              )}
            </div>
          </div>
        ))}

        {isWaiting && (
          <div className="flex">
            <div className="w-8 h-8 rounded-full bg-gray-700 flex items-center justify-center mr-3 flex-shrink-0 mt-1">
              <Bot className="w-4 h-4 text-gray-400" />
            </div>
            <div className="bg-gray-800 rounded-2xl rounded-tl-none px-5 py-4 max-w-[85%] border border-gray-700 flex items-center space-x-2">
              <div className="w-2 h-2 bg-gray-500 rounded-full animate-bounce"></div>
              <div
                className="w-2 h-2 bg-gray-500 rounded-full animate-bounce"
                style={{ animationDelay: "0.2s" }}
              ></div>
              <div
                className="w-2 h-2 bg-gray-500 rounded-full animate-bounce"
                style={{ animationDelay: "0.4s" }}
              ></div>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      <div className="p-4 bg-gray-800 border-t border-gray-700">
        <form
          onSubmit={sendMessage}
          className="max-w-4xl mx-auto relative flex items-center"
        >
          <input
            type="text"
            value={chatInput}
            onChange={(e) => setChatInput(e.target.value)}
            placeholder="Type a message or command..."
            className="w-full bg-gray-900 border border-gray-600 rounded-full py-3 pl-5 pr-12 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-colors"
            autoComplete="off"
            autoFocus
          />
          <button
            type="submit"
            disabled={isWaiting || !chatInput.trim()}
            className="absolute right-2 top-1/2 transform -translate-y-1/2 w-10 h-10 rounded-full bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center transition-colors"
          >
            <Send className="w-4 h-4" />
          </button>
        </form>
      </div>
    </div>
  );

  const renderSettings = () => (
    <div className="flex-1 overflow-y-auto p-6 md:p-10">
      <h2 className="text-3xl font-bold mb-6">Settings</h2>

      <form
        onSubmit={saveSettings}
        className="max-w-2xl bg-gray-800 rounded-lg p-6 shadow border border-gray-700"
      >
        <h3 className="text-xl font-bold mb-4 border-b border-gray-700 pb-2">
          LLM Configuration
        </h3>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">
              Provider
            </label>
            <select
              value={localConfig.provider}
              onChange={(e) =>
                setLocalConfig({ ...localConfig, provider: e.target.value })
              }
              className="w-full bg-gray-900 border border-gray-600 rounded p-2 focus:outline-none focus:border-blue-500"
            >
              <option value="ollama">Ollama (Local)</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="gemini">Google Gemini</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">
              Model Name
            </label>
            <input
              type="text"
              value={localConfig.model}
              onChange={(e) =>
                setLocalConfig({ ...localConfig, model: e.target.value })
              }
              className="w-full bg-gray-900 border border-gray-600 rounded p-2 focus:outline-none focus:border-blue-500"
              placeholder="e.g. llama3.2"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">
              Base URL (Optional)
            </label>
            <input
              type="text"
              value={localConfig.base_url || ""}
              onChange={(e) =>
                setLocalConfig({ ...localConfig, base_url: e.target.value })
              }
              className="w-full bg-gray-900 border border-gray-600 rounded p-2 focus:outline-none focus:border-blue-500"
              placeholder="http://localhost:11434"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">
              API Key
            </label>
            <input
              type="password"
              value={localConfig.api_key || ""}
              onChange={(e) =>
                setLocalConfig({ ...localConfig, api_key: e.target.value })
              }
              className="w-full bg-gray-900 border border-gray-600 rounded p-2 focus:outline-none focus:border-blue-500"
              placeholder="sk-..."
            />
            <p className="text-xs text-gray-500 mt-1">
              Leave blank to use environment variables.
            </p>
          </div>

          <div className="pt-4 border-t border-gray-700">
            <label className="block text-sm font-medium text-gray-400 mb-1">
              Scripts Directory
            </label>
            <input
              type="text"
              value={localConfig.scripts_dir || ""}
              onChange={(e) =>
                setLocalConfig({ ...localConfig, scripts_dir: e.target.value })
              }
              className="w-full bg-gray-900 border border-gray-600 rounded p-2 focus:outline-none focus:border-blue-500"
              placeholder="scripts"
            />
          </div>
        </div>

        <div className="mt-8 flex items-center justify-end space-x-4">
          {showSaveSuccess && (
            <span className="text-green-500 text-sm flex items-center">
              <Check className="w-4 h-4 mr-1" /> Saved successfully
            </span>
          )}
          <button
            type="submit"
            className="bg-blue-600 hover:bg-blue-700 px-6 py-2 rounded font-medium transition-colors"
          >
            Save Configuration
          </button>
        </div>
      </form>
    </div>
  );

  return (
    <div className="bg-gray-900 text-white h-screen flex overflow-hidden">
      {/* Sidebar */}
      <aside className="w-64 bg-gray-800 hidden md:flex flex-col shadow-lg z-10">
        <div className="h-16 flex items-center px-6 border-b border-gray-700">
          <Shield className="text-blue-500 w-8 h-8 mr-3" />
          <h1 className="text-xl font-bold tracking-wider">IRon AI</h1>
        </div>
        <nav className="flex-1 py-4">
          <ul className="space-y-1">
            <li>
              <button
                onClick={() => setActiveTab("dashboard")}
                className={`w-full text-left px-6 py-3 flex items-center transition-colors ${activeTab === "dashboard" ? "bg-gray-700 border-l-4 border-blue-500" : "hover:bg-gray-700 border-l-4 border-transparent"}`}
              >
                <LayoutDashboard className="w-5 h-5 mr-3" /> Dashboard
              </button>
            </li>
            <li>
              <button
                onClick={() => setActiveTab("chat")}
                className={`w-full text-left px-6 py-3 flex items-center transition-colors ${activeTab === "chat" ? "bg-gray-700 border-l-4 border-blue-500" : "hover:bg-gray-700 border-l-4 border-transparent"}`}
              >
                <MessageSquare className="w-5 h-5 mr-3" /> Chat
              </button>
            </li>
            <li>
              <button
                onClick={() => setActiveTab("settings")}
                className={`w-full text-left px-6 py-3 flex items-center transition-colors ${activeTab === "settings" ? "bg-gray-700 border-l-4 border-blue-500" : "hover:bg-gray-700 border-l-4 border-transparent"}`}
              >
                <Settings className="w-5 h-5 mr-3" /> Settings
              </button>
            </li>
          </ul>
        </nav>
        <div className="p-4 border-t border-gray-700 text-sm text-gray-400">
          <span className="flex items-center">
            {status.isOnline ? (
              <>
                <span className="w-2 h-2 rounded-full bg-green-500 mr-2"></span>{" "}
                Online
              </>
            ) : (
              <>
                <span className="w-2 h-2 rounded-full bg-red-500 mr-2"></span>{" "}
                Offline
              </>
            )}
          </span>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col relative overflow-hidden">
        {/* Mobile Header */}
        <header className="h-16 bg-gray-800 border-b border-gray-700 flex items-center px-4 md:hidden">
          <Shield className="text-blue-500 w-6 h-6 mr-2" />
          <h1 className="text-lg font-bold">IRon AI</h1>
          <div className="ml-auto flex space-x-4">
            <button
              onClick={() => setActiveTab("dashboard")}
              className={
                activeTab === "dashboard" ? "text-blue-500" : "text-gray-400"
              }
            >
              <LayoutDashboard className="w-5 h-5" />
            </button>
            <button
              onClick={() => setActiveTab("chat")}
              className={
                activeTab === "chat" ? "text-blue-500" : "text-gray-400"
              }
            >
              <MessageSquare className="w-5 h-5" />
            </button>
            <button
              onClick={() => setActiveTab("settings")}
              className={
                activeTab === "settings" ? "text-blue-500" : "text-gray-400"
              }
            >
              <Settings className="w-5 h-5" />
            </button>
          </div>
        </header>

        {/* Dynamic View */}
        {activeTab === "dashboard" && renderDashboard()}
        {activeTab === "chat" && renderChat()}
        {activeTab === "settings" && renderSettings()}
      </main>
    </div>
  );
}
