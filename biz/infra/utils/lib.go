package utils

func Convert[T any](in any) (out T, ok bool) {
	if v, ok := in.(T); ok {
		return v, true
	}
	return
}
