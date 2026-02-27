package autoload

// Import all middleware subpackages for side-effect registration.
import (
	_ "iron/middlewares/alarm"
	_ "iron/middlewares/calendar"
	_ "iron/middlewares/codingtools"
	_ "iron/middlewares/cron"
	_ "iron/middlewares/device"
	_ "iron/middlewares/email"
	_ "iron/middlewares/emmetbridge"
	_ "iron/middlewares/greeting"
	_ "iron/middlewares/intentcompressor"
	_ "iron/middlewares/localcache"
	_ "iron/middlewares/notes"
	_ "iron/middlewares/pytools"
	_ "iron/middlewares/slack"
	_ "iron/middlewares/tokenbudget"
	_ "iron/middlewares/trashcleanner"
	_ "iron/middlewares/weather"
)
