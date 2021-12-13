package terramate

type StackMetadata struct {
	Name string
	Path string
}

type Metadata struct {
	Stacks []StackMetadata
}

func LoadMetadata(basedir string) (Metadata, error) {
	return Metadata{}, nil
}
