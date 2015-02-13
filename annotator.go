package logging

// An Annotator is a kind of LeveledBackend that adds some annotations
// to a message in flight
type Annotator struct {
	Extras map[string]Annotation
	backend LeveledBackend
}

func NewAnnotator(b LeveledBackend) *Annotator {
	return &Annotator{
		Extras: map[string]Annotation{},
		backend: b,
	}
}

func (a *Annotator) Add(key string, val interface{}) {
	a.Extras[key] = Annotation{Key: key, Value: val}
}

func (a *Annotator) Backend() LeveledBackend {
	if a.backend == nil {
		return defaultBackend
	} else {
		return a.backend
	}
}

// most of these are pass-thru
func (a *Annotator) GetLevel(mod string) Level {
	return a.Backend().GetLevel(mod)
}

func (a *Annotator) SetLevel(lvl Level, mod string) {
	a.Backend().SetLevel(lvl, mod)
}

func (a *Annotator) IsEnabledFor(lvl Level, mod string) bool {
	return a.Backend().IsEnabledFor(lvl, mod)
}

func (a *Annotator) Log(lvl Level, depth int, rec *Record) error {
	// here's the only place we do our work
	for _, ex := range a.Extras {
		rec.Annotations = append(rec.Annotations, ex)
	}
	return a.Backend().Log(lvl, depth+1, rec)
}

