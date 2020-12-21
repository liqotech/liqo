package forge

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	LiqoOutgoingKey = "virtualkubelet.liqo.io/outgoing"
	LiqoIncomingKey = "virtualkubelet.liqo.io/incoming"
)

var (
	LiqoNodeName = func() string {
		if forger.virtualNodeName == nil {
			return ""
		}
		return forger.virtualNodeName.Value().ToString()
	}
)

func (f *apiForger) forgeForeignMeta(homeMeta, foreignMeta *metav1.ObjectMeta, foreignNamespace, reflectionType string) {
	forgeObjectMeta(homeMeta, foreignMeta)

	foreignMeta.Namespace = foreignNamespace
	foreignMeta.Labels[reflectionType] = LiqoNodeName()
}

func (f *apiForger) forgeHomeMeta(foreignMeta, homeMeta *metav1.ObjectMeta, homeNamespace, reflectionType string) {
	forgeObjectMeta(foreignMeta, homeMeta)

	homeMeta.Namespace = homeNamespace
	homeMeta.Labels[reflectionType] = LiqoNodeName()
}

func forgeObjectMeta(inMeta, outMeta *metav1.ObjectMeta) {
	outMeta.Name = inMeta.Name

	if outMeta.Annotations == nil {
		outMeta.Annotations = make(map[string]string)
	}
	for k, v := range inMeta.Annotations {
		outMeta.Annotations[k] = v
	}

	if outMeta.Labels == nil {
		outMeta.Labels = make(map[string]string)
	}
	for k, v := range inMeta.Labels {
		outMeta.Labels[k] = v
	}
}
