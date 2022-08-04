package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractSecondLevelDomain(t *testing.T) {
	assert.Equal(t, "", extractSecondLevelDomain("com"))
	assert.Equal(t, "domain.com", extractSecondLevelDomain("domain.com"))
	assert.Equal(t, "domain.com", extractSecondLevelDomain("sub.domain.com"))
	assert.Equal(t, "domain.com", extractSecondLevelDomain("sub.sub.domain.com"))
}
