package beacon

import "golang.org/x/sync/errgroup"

func sequential(fs ...func() error) (err error) {
	for _, f := range fs {
		err = f()
		if err != nil {
			break
		}
	}
	return
}

func concurrent(fs ...func() error) error {
	g := new(errgroup.Group)
	for _, f := range fs {
		g.Go(f)
	}
	return g.Wait()
}
