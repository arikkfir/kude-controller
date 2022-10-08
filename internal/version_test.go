package internal

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestGetVersion(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(GetVersion()).To(Equal(version))
}
