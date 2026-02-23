package autoload

// Import all middleware subpackages for side-effect registration.
import (
	_ "iron/middlewares/trashcleanner"
)
