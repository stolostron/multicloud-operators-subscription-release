package controller

import (
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/controller/subscription"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, subscription.Add)
}
