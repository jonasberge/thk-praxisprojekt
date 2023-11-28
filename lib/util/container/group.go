package container

type Group struct {
	Containers []*Container
	CallerFunc string
}

func NewGroup(containers ...*Container) *Group {
	return &Group{
		Containers: containers,
		CallerFunc: callerFuncName(),
	}
}
