package learn

// SnapshotHardModelsForTest captures every game's hard-model switch and
// returns a function that puts them back.
//
// The switches are process-wide, as they have to be for a console change to
// reach every table. That makes them shared state between tests: one that
// turns a model on and leaves it there - app.NewMobile turns on every model
// the binary ships - changes what the next test's AI seats play and what its
// switch reports, in a test that looks unrelated.
//
// Exported because the tests that need it are in other packages: internal/app
// builds whole nodes, and a mobile node switches its models on at boot.
func SnapshotHardModelsForTest() (restore func()) {
	was := make(map[*hardModel]bool, len(hardModels))
	for _, m := range hardModels {
		was[m] = m.enabled.Load()
	}
	return func() {
		for m, on := range was {
			m.enabled.Store(on)
		}
	}
}
