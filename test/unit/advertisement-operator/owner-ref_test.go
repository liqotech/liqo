package advertisement_operator

import (
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	pkg "github.com/liqoTech/liqo/pkg/advertisement-operator"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestGetDeployOwnerRef(t *testing.T) {
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "deploy1",
			UID:  "id1",
		},
	}
	ownerRef := pkg.GetOwnerReference(deploy)
	assert.Len(t, ownerRef, 1)
	assert.Equal(t, deploy.Kind, ownerRef[0].Kind)
	assert.Equal(t, deploy.APIVersion, ownerRef[0].APIVersion)
	assert.Equal(t, deploy.Name, ownerRef[0].Name)
	assert.Equal(t, deploy.UID, ownerRef[0].UID)
}

func TestGetAdvertisementOwnerRef(t *testing.T) {
	adv := &advtypes.Advertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Advertisement",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "adv1",
			UID:  "id2",
		},
	}
	ownerRef := pkg.GetOwnerReference(adv)
	assert.Len(t, ownerRef, 1)
	assert.Equal(t, adv.Kind, ownerRef[0].Kind)
	assert.Equal(t, adv.APIVersion, ownerRef[0].APIVersion)
	assert.Equal(t, adv.Name, ownerRef[0].Name)
	assert.Equal(t, adv.UID, ownerRef[0].UID)
}
