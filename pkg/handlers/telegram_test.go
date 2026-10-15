package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCreatedComment(t *testing.T) {
	t.Parallel()

	assert.True(t, isCreatedComment(""))
	assert.True(t, isCreatedComment("created"))
	assert.True(t, isCreatedComment("Created"))
	assert.False(t, isCreatedComment("deleted"))
	assert.False(t, isCreatedComment("edited"))
}
