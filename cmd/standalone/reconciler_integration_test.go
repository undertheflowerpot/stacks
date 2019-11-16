package main

import (
	"github.com/docker/stacks/pkg/reconciler/reconciler"
)

var _ = reconciler.InitAlgSecret
var _ = reconciler.InitAlgConfig
var _ = reconciler.InitAlgNetwork
var _ = reconciler.InitAlgService
