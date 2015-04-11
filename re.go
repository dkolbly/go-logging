package logging

type Annotater interface {
	Annotate(*Record)
}

// an annotator stack level
type stacked struct {
	lower, higher Annotater
}

func (s stacked) Annotate(rec *Record) {
	s.lower.Annotate(rec)
	s.higher.Annotate(rec)
}

// Re returns a new logger that annotates log messages with the
// appropriate context (as defined by the Annotater)
func (l *Logger) Re(a Annotater) *Logger {

	// if there's an existing annotator, stack them together
	if l.annotater != nil {
		a = stacked{l.annotater, a}
	}

	lcopy := *l
	lcopy.annotater = a
	return &lcopy
}
