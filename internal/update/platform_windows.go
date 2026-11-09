package update

import (
	"context"
	"errors"
)

func preparePlatformUpdate(ctx context.Context, target string, downloaded Download, args []string) (*Installer, error) {
	return nil, errors.New("update package does not match Windows")
}
