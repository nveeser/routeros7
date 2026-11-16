package routerosv7

import (
	"context"

	"github.com/nveeser/routeros7/session"
	"golang.org/x/sync/errgroup"
)

type Resource interface {
	Requests() []session.Requester
}

type SingletonResource interface {
	Resource
	SingletonPath() string
}

type IdResource interface {
	CollectionPath() string
}

type NameResource interface {
	IdResource
	Name() string
}

func Read(ctx context.Context, c *session.Client, result Resource) error {
	reqs := result.Requests()
	if len(reqs) == 1 {
		return c.Print(ctx, reqs[0])
	}
	g, cctx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for _, req := range reqs {
		req := req
		g.Go(func() error {
			return c.Print(cctx, req)
		})
	}
	return g.Wait()
}

func ReadSlice[T IdResource](ctx context.Context, c *session.Client, result *[]T) error {
	return c.Print(ctx, requestSlice(result))
}

func ReadId[T IdResource](ctx context.Context, c *session.Client, result *T, id string) error {
	return c.Print(ctx, requestId(result, id))
}

func ReadName[T NameResource](ctx context.Context, c *session.Client, result *T, name string) error {
	return c.Print(ctx, requestId(result, name))
}

func requestSingle[T SingletonResource](result T) session.Requester {
	return &session.Request[T]{Command: result.SingletonPath(), Result: result}
}

func requestId[T IdResource](result *T, id string) session.Requester {
	var r T
	return &session.Request[*T]{Command: r.CollectionPath() + "/" + id, Result: result}
}

func requestSlice[T IdResource](result *[]T) session.Requester {
	var r T
	return &session.Request[*[]T]{Command: r.CollectionPath(), Result: result}
}

func requests(r session.Requester) []session.Requester {
	return []session.Requester{r}
}

func concat(s ...session.Requester) (out []session.Requester) {
	for _, ss := range s {
		out = append(out, ss)
	}
	return
}

func concatAll(s ...[]session.Requester) (out []session.Requester) {
	for _, ss := range s {
		out = append(out, ss...)
	}
	return
}

type rr struct {
	session.Requester
}

func (r *rr) Requests() []session.Requester {
	return []session.Requester{r}
}

func asResource[T IdResource](result *[]T) Resource {
	var r T
	return &rr{
		&session.Request[*[]T]{Command: r.CollectionPath(), Result: result},
	}
}
