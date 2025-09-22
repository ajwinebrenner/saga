package errs

type Static string

func (e Static) Error() string {
	return string(e)
}
