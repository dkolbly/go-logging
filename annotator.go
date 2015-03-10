package logging

// An Annotator is a kind of Backend that adds some annotations
// to a message in flight
type Annotator struct {
	Extras map[string]Annotation
	DynamicExtras func() []Annotation
	backend Backend
}

func NewAnnotator(b Backend) *Annotator {
	return &Annotator{
		Extras: map[string]Annotation{},
		backend: b,
	}
}

func (a *Annotator) Add(key string, val interface{}) {
	a.Extras[key] = Annotation{Key: key, Value: val}
}

func (a *Annotator) Backend() Backend {
	if a.backend == nil {
		return defaultBackend
	} else {
		return a.backend
	}
}

func (a *Annotator) Log(lvl Level, depth int, rec *Record) error {
	// here's the only place we do our work
	for _, ex := range a.Extras {
		rec.Annotations = append(rec.Annotations, ex)
	}
	if a.DynamicExtras != nil {
		x := a.DynamicExtras()
		if x != nil {
			for _, ex := range x {
				rec.Annotations = append(rec.Annotations, ex)
			}
		}
	}
	return a.Backend().Log(lvl, depth+1, rec)
}

