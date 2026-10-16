package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldNotifyPRComment(t *testing.T) {
	t.Parallel()

	assert.True(t, shouldNotifyPRComment(""))
	assert.True(t, shouldNotifyPRComment("created"))
	assert.True(t, shouldNotifyPRComment("Created"))
	assert.True(t, shouldNotifyPRComment("edited"))
	assert.False(t, shouldNotifyPRComment("deleted"))
	assert.False(t, shouldNotifyPRComment("delete"))
}
