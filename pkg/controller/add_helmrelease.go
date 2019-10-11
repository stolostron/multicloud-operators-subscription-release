package controller

import (
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/controller/helmrelease"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, helmrelease.Add)
}
