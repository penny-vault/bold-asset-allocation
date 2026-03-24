package baa_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBoldAssetAllocation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bold Asset Allocation Suite")
}
