package main

import "context"

type Main struct{}

func (m *Main) Encapsulate(ctx context.Context) error {
	_ = m.Fail(ctx)
	return nil
}

func (*Main) Fail(ctx context.Context) error {
	_, err := dag.Container().From("nixos/nix").WithExec([]string{"false"}).Sync(ctx)
	return err
}
