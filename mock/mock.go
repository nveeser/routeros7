// mocks.go

package mock

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type RouterOSAPI struct {
	mock.Mock
}

func (m *RouterOSAPI) Auth(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

func (m *RouterOSAPI) Print(ctx context.Context, command string) (interface{}, error) {
	args := m.Called(ctx, command)
	return args.Get(0), args.Error(1)
}

func (m *RouterOSAPI) Add(
	ctx context.Context, command string, payload []byte,
) (interface{}, error) {
	args := m.Called(ctx, command, payload)
	return args.Get(0), args.Error(1)
}

func (m *RouterOSAPI) Set(
	ctx context.Context, command string, payload []byte,
) (interface{}, error) {
	args := m.Called(ctx, command, payload)
	return args.Get(0), args.Error(1)
}

func (m *RouterOSAPI) Remove(ctx context.Context, command string) (interface{}, error) {
	args := m.Called(ctx, command)
	return args.Get(0), args.Error(1)
}

func (m *RouterOSAPI) Run(
	ctx context.Context, command string, payload []byte,
) (interface{}, error) {
	args := m.Called(ctx, command, payload)
	return args.Get(0), args.Error(1)
}
