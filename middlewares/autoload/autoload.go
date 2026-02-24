package autoload

// Import all middleware subpackages for side-effect registration.
import (
	_ "iron/middlewares/codingtools"
	_ "iron/middlewares/emmetbridge"
	_ "iron/middlewares/greeting"
	_ "iron/middlewares/intentcompressor"
	_ "iron/middlewares/localcache"
	_ "iron/middlewares/tokenbudget"
	_ "iron/middlewares/trashcleanner"
)
