package repotest

// import (
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// 	"github.com/suiteserve/suiteserve/internal/repo"
// 	"testing"
// )
//
// func TestRepo_Suite(t *testing.T) {
// 	r := Open(t)
// 	_, err := r.Suite("nonexistent")
// 	assert.True(t, isNotFound(err), "want not found")
//
// 	want := repo.Suite{
// 		Name: "test",
// 	}
// 	id, err := r.InsertSuite(want)
// 	require.Nil(t, err)
// 	want.Id = id
//
// 	got, err := r.Suite(id)
// 	require.Nil(t, err)
// 	assert.Equal(t, want, got)
// }
