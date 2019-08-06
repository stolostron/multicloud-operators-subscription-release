package controller

import (
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/controller/subscriptionrelease"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, subscriptionrelease.Add)
}
