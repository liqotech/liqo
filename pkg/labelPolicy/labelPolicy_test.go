package labelPolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var nodes = &v1.NodeList{
	Items: []v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"test1": "true",
					"test2": "true",
					"test3": "true",
					"test4": "false",
					"test6": "",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"test1": "false",
					"test2": "true",
					"test4": "false",
					"test6": "",
				},
			},
		},
	},
}

func TestAnyTrue(t *testing.T) {
	policy := GetInstance(LabelPolicyAnyTrue)
	assert.NotNil(t, policy)

	val, insert := policy.Process(nodes, "test1")
	assert.EqualValues(t, "true", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test2")
	assert.EqualValues(t, "true", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test3")
	assert.EqualValues(t, "true", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test4")
	assert.EqualValues(t, "false", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test5")
	assert.EqualValues(t, "false", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test6")
	assert.EqualValues(t, "true", val)
	assert.Equal(t, insert, true)
}

func TestAllTrue(t *testing.T) {
	policy := GetInstance(LabelPolicyAllTrue)
	assert.NotNil(t, policy)

	val, insert := policy.Process(nodes, "test1")
	assert.EqualValues(t, "false", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test2")
	assert.EqualValues(t, "true", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test3")
	assert.EqualValues(t, "false", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test4")
	assert.EqualValues(t, "false", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test5")
	assert.EqualValues(t, "false", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test6")
	assert.EqualValues(t, "true", val)
	assert.Equal(t, insert, true)
}

func TestAnyTrueNoLabelIfFalse(t *testing.T) {
	policy := GetInstance(LabelPolicyAnyTrueNoLabelIfFalse)
	assert.NotNil(t, policy)

	val, insert := policy.Process(nodes, "test1")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test2")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test3")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test4")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, false)

	val, insert = policy.Process(nodes, "test5")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, false)

	val, insert = policy.Process(nodes, "test6")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, true)
}

func TestAllTrueNoLabelIfFalse(t *testing.T) {
	policy := GetInstance(LabelPolicyAllTrueNoLabelIfFalse)
	assert.NotNil(t, policy)

	val, insert := policy.Process(nodes, "test1")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, false)

	val, insert = policy.Process(nodes, "test2")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, true)

	val, insert = policy.Process(nodes, "test3")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, false)

	val, insert = policy.Process(nodes, "test4")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, false)

	val, insert = policy.Process(nodes, "test5")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, false)

	val, insert = policy.Process(nodes, "test6")
	assert.EqualValues(t, "", val)
	assert.Equal(t, insert, true)
}
