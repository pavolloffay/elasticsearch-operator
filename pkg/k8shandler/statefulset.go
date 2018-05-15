package k8shandler

import (
	"fmt"

	"github.com/sirupsen/logrus"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	apps "k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	//	"encoding/json"
)

const (
	elasticsearchCertsPath    = "/etc/elasticsearch/secret"
	clusterHealthURL          = "/_nodes/_local"
	elasticsearchConfigPath   = "/usr/share/java/elasticsearch/config"
	elasticsearchDefaultImage = "docker.io/t0ffel/elasticsearch5"
	defaultMasterCPULimit     = "100m"
	defaultMasterCPURequest   = "100m"
	defaultCPULimit           = "4000m"
	defaultCPURequest         = "100m"
	defaultMemoryLimit        = "4Gi"
	defaultMemoryRequest      = "1Gi"
)

type ESClusterNodeConfig struct {
	ClusterName     string
	StatefulSetName string
	NodeType        string
	// StorageType		string
	ESNodeSpec v1alpha1.ElasticsearchNode
}

func constructNodeConfig(dpl *v1alpha1.Elasticsearch, esNode v1alpha1.ElasticsearchNode) (ESClusterNodeConfig, error) {
	nodeCfg := ESClusterNodeConfig{}
	nodeCfg.StatefulSetName = fmt.Sprintf("%s-%s", dpl.Name, esNode.NodeRole)
	nodeCfg.ClusterName = dpl.Name
	nodeCfg.NodeType = esNode.NodeRole
	nodeCfg.ESNodeSpec = esNode

	return nodeCfg, nil
}

func (cfg *ESClusterNodeConfig) getReplicas() int32 {
	return cfg.ESNodeSpec.Replicas
}

func (cfg *ESClusterNodeConfig) isNodeMaster() string {
	if cfg.NodeType == "clientdatamaster" || cfg.NodeType == "master" {
		return "true"
	}
	return "false"
}

func (cfg *ESClusterNodeConfig) isNodeData() string {
	if cfg.NodeType == "clientdatamaster" || cfg.NodeType == "clientdata" || cfg.NodeType == "data" {
		return "true"
	}
	return "false"
}

func (cfg *ESClusterNodeConfig) isNodeClient() string {
	if cfg.NodeType == "clientdatamaster" || cfg.NodeType == "clientdata" || cfg.NodeType == "client" {
		return "true"
	}
	return "false"
}

func (cfg *ESClusterNodeConfig) getLabels() map[string]string {
	return map[string]string{
		"component":      fmt.Sprintf("elasticsearch-%s", cfg.ClusterName),
		"es-node-role":   cfg.NodeType,
		"es-node-client": cfg.isNodeClient(),
		"es-node-data":   cfg.isNodeData(),
		"es-node-master": cfg.isNodeMaster(),
		"cluster":        cfg.ClusterName,
	}
}

func (cfg *ESClusterNodeConfig) getReadinessProbe() v1.Probe {
	return v1.Probe{
		TimeoutSeconds:      30,
		InitialDelaySeconds: 10,
		FailureThreshold:    15,
		Handler: v1.Handler{
			TCPSocket: &v1.TCPSocketAction{
				Port: intstr.FromInt(9300),
			},
		},
	}
}

func (cfg *ESClusterNodeConfig) getAffinity() v1.Affinity {
	return v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "role",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{cfg.NodeType},
								},
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
	}
}

func (cfg *ESClusterNodeConfig) getEnvVars() []v1.EnvVar {
	return []v1.EnvVar{
		v1.EnvVar{
			Name: "NAMESPACE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		v1.EnvVar{
			Name:  "CLUSTER_NAME",
			Value: cfg.ClusterName,
		},
		v1.EnvVar{
			Name: "NODE_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		v1.EnvVar{
			Name:  "IS_MASTER",
			Value: cfg.isNodeMaster(),
		},
		v1.EnvVar{
			Name:  "HAS_DATA",
			Value: cfg.isNodeData(),
		},
		v1.EnvVar{
			Name:  "SERVICE_DNS",
			Value: fmt.Sprintf("%s-cluster", cfg.ClusterName),
		},
		v1.EnvVar{
			Name:  "INSTANCE_RAM",
			Value: cfg.getInstanceRAM(),
		},
		v1.EnvVar{
			Name:  "NODE_QUORUM",
			Value: "1",
		},
		v1.EnvVar{
			Name:  "RECOVER_EXPECTED_NODES",
			Value: "1",
		},
		v1.EnvVar{
			Name:  "RECOVER_AFTER_TIME",
			Value: "5m",
		},
		v1.EnvVar{
			Name:  "KIBANA_INDEX_MODE",
			Value: "5m",
		},
		v1.EnvVar{
			Name:  "ALLOW_CLUSTER_READER",
			Value: "false",
		},
	}
}

func (cfg *ESClusterNodeConfig) getInstanceRAM() string {
	memory := cfg.ESNodeSpec.Resources.Limits.Memory()
	if !memory.IsZero() {
		return memory.String()
	}
	return defaultMemoryLimit
}

func (cfg *ESClusterNodeConfig) getResourceRequirements() v1.ResourceRequirements {
	limitCPU := cfg.ESNodeSpec.Resources.Limits.Cpu()
	if limitCPU.IsZero() {
		CPU, _ := resource.ParseQuantity(defaultCPULimit)
		limitCPU = &CPU
	}
	limitMem, _ := resource.ParseQuantity(cfg.getInstanceRAM())
	requestCPU := cfg.ESNodeSpec.Resources.Requests.Cpu()
	if requestCPU.IsZero() {
		CPU, _ := resource.ParseQuantity(defaultCPURequest)
		requestCPU = &CPU
	}
	requestMem := cfg.ESNodeSpec.Resources.Requests.Memory()
	if requestMem.IsZero() {
		Mem, _ := resource.ParseQuantity(defaultMemoryRequest)
		requestMem = &Mem
	}
	logrus.Infof("Using  memory limit: %v, for node %v", limitMem.String(), cfg.StatefulSetName)

	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			"cpu":    *limitCPU,
			"memory": limitMem,
		},
		Requests: v1.ResourceList{
			"cpu":    *requestCPU,
			"memory": *requestMem,
		},
	}

}

func (cfg *ESClusterNodeConfig) getContainer() v1.Container {
	probe := cfg.getReadinessProbe()
	return v1.Container{
		Name:            cfg.StatefulSetName,
		Image:           elasticsearchDefaultImage,
		ImagePullPolicy: "Always",
		Env:             cfg.getEnvVars(),
		Ports: []v1.ContainerPort{
			v1.ContainerPort{
				Name:          "cluster",
				ContainerPort: 9300,
				Protocol:      v1.ProtocolTCP,
			},
			v1.ContainerPort{
				Name:          "restapi",
				ContainerPort: 9200,
				Protocol:      v1.ProtocolTCP,
			},
		},
		ReadinessProbe: &probe,
		LivenessProbe:  &probe,
		VolumeMounts: []v1.VolumeMount{
			v1.VolumeMount{
				Name:      "es-data",
				MountPath: "/elasticsearch/persistent",
			},
			v1.VolumeMount{
				Name:      "certificates",
				MountPath: elasticsearchCertsPath,
			},
		},
		Resources: cfg.getResourceRequirements(),
	}
}

// CreateDataNodeDeployment creates the data node deployment
func (cfg *ESClusterNodeConfig) constructNodeStatefulSet(namespace string) (*apps.StatefulSet, error) {

	secretName := fmt.Sprintf("%s-certs", cfg.ClusterName)

	// Check if StatefulSet exists

	// FIXME: remove hardcode
	volumeSize, _ := resource.ParseQuantity("1Gi")
	storageClass := "default"

	affinity := cfg.getAffinity()
	replicas := cfg.getReplicas()

	statefulSet := statefulSet(cfg.StatefulSetName, namespace)
	statefulSet.Spec = apps.StatefulSetSpec{
		Replicas:    &replicas,
		ServiceName: cfg.StatefulSetName,
		Selector: &metav1.LabelSelector{
			MatchLabels: cfg.getLabels(),
		},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: cfg.getLabels(),
			},
			Spec: v1.PodSpec{
				Affinity: &affinity,
				Containers: []v1.Container{
					cfg.getContainer(),
				},
				Volumes: []v1.Volume{
					v1.Volume{
						Name: "certificates",
						VolumeSource: v1.VolumeSource{
							Secret: &v1.SecretVolumeSource{
								SecretName: secretName,
							},
						},
					},
				},
				// ImagePullSecrets: TemplateImagePullSecrets(imagePullSecrets),
			},
		},
		VolumeClaimTemplates: []v1.PersistentVolumeClaim{
			v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "es-data",
					Labels: cfg.getLabels(),
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{
						v1.ReadWriteOnce,
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: volumeSize,
						},
					},
				},
			},
		},
	}

	if storageClass != "default" {
		statefulSet.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{
			"volume.beta.kubernetes.io/storage-class": storageClass,
		}
	}
	// sset, _ := json.Marshal(statefulSet)
	// s := string(sset[:])

	// logrus.Infof(s)

	return statefulSet, nil
}

func statefulSet(ssName string, namespace string) *apps.StatefulSet {
	return &apps.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ssName,
			Namespace: namespace,
		},
	}
}
