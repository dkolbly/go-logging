package logging

// Re returns a new logger that annotates log messages with the
// appropriate context (as defined by the Annotater)
func (l *Logger) Re(a Annotater) *Logger {
	lcopy := *l
	lcopy.annotater = a
	return &lcopy
}

type Annotater interface {
	Annotate(*Record)
}
