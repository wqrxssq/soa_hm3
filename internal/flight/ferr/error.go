package ferr

type Kind int

const (
	KindInvalid Kind = iota + 1
	KindNotFound
	KindExhausted
	KindPrecondition
	KindAlreadyExists
)

type Error struct {
	Kind    Kind
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func Invalid(message string) error {
	return &Error{Kind: KindInvalid, Message: message}
}

func NotFound(message string) error {
	return &Error{Kind: KindNotFound, Message: message}
}

func Exhausted(message string) error {
	return &Error{Kind: KindExhausted, Message: message}
}

func Precondition(message string) error {
	return &Error{Kind: KindPrecondition, Message: message}
}

func AlreadyExists(message string) error {
	return &Error{Kind: KindAlreadyExists, Message: message}
}
