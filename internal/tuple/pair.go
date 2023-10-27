package tuple

type T2[V1, V2 any] struct {
	V1 V1
	V2 V2
}

func (t2 T2[V1, V2]) Values() (V1, V2) {
	return t2.V1, t2.V2
}

func New2[V1, V2 any](v1 V1, v2 V2) T2[V1, V2] {
	return T2[V1, V2]{v1, v2}
}
