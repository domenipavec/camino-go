package diary

import "encoding/gob"

type Flash struct {
	Type    string
	Message string
}

func FlashInfo(msg string) Flash {
	return Flash{
		Type:    "info",
		Message: msg,
	}
}

func FlashError(msg string) Flash {
	return Flash{
		Type:    "danger",
		Message: msg,
	}
}

func init() {
	gob.Register(&Flash{})
}
