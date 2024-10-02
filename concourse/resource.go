package main

import (
	"concourse/internal/dagger"
	"context"
	"time"
)

// A resource represents an external versioned asset to be published or
// consumed by your pipeline.
type Resource struct {
	// Must be nil when installed onto a Pipeline.
	// +private
	Concourse *Concourse

	Name      string
	Container *dagger.Container
	Source    dagger.JSON
}

func (res *Resource) finiteStream(from dagger.JSON) Stream[*ResourceVersion] {
	return &SliceStream[*ResourceVersion]{
		load: func(ctx context.Context) ([]*ResourceVersion, error) {
			vs, err := res.Check(ctx, from)
			if err != nil {
				return nil, err
			}
			if len(vs) == 0 {
				return vs, nil
			}
			if from == "" {
				// just return the last version, to cope with registry-image behavior
				vs = vs[len(vs)-1:]
			}
			return vs, nil
		},
	}
}

func (res *Resource) infiniteStream(from dagger.JSON) Stream[*ResourceVersion] {
	return Chain(res.finiteStream(from), func(last *ResourceVersion) (Stream[*ResourceVersion], error) {
		time.Sleep(checkInterval) // TODO: respect config
		from := from
		if last != nil {
			from = last.Version
		}
		return res.finiteStream(from), nil
	})
}
