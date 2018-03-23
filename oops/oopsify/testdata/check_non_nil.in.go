package foo

func f() error {
	return nil
}

func g() {
	if err := f(); err != nil {
	}
}
