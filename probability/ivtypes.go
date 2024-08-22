package probability

type VolType struct {
	Name string
	Vol  float64
}

type cacheKey struct {
	spreadID  string
	volType   string
	modelName string
}
