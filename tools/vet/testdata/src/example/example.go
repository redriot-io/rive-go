package example

type Image struct{ X, Y float64 }
type Artboard struct{ Name string }
type Node struct{ Name string }

func flagged() {
	_ = &Image{}    // want `use NewImage\(\) instead of &Image\{\}`
	_ = &Artboard{} // want `use NewArtboard\(\) instead of &Artboard\{\}`
}

func notFlagged() {
	// Node is not in the types list — no diagnostic.
	_ = &Node{}
	// Struct with fields explicitly set — still flagged (constructor should set defaults).
	_ = &Image{X: 1.0} // want `use NewImage\(\) instead of &Image\{\}`
}
