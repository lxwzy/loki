package manifests

import (
	"fmt"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func configureServiceCA(podSpec *corev1.PodSpec, caBundleName string) error {
	secretVolumeSpec := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: caBundleName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: caBundleName,
						},
					},
				},
			},
		},
	}

	secretContainerSpec := corev1.Container{
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      caBundleName,
				ReadOnly:  false,
				MountPath: caBundleDir,
			},
		},
	}

	if err := mergo.Merge(podSpec, secretVolumeSpec, mergo.WithAppendSlice); err != nil {
		return kverrors.Wrap(err, "failed to merge volumes")
	}

	if err := mergo.Merge(&podSpec.Containers[0], secretContainerSpec, mergo.WithAppendSlice); err != nil {
		return kverrors.Wrap(err, "failed to merge container")
	}

	return nil
}

func configureGRPCServicePKI(podSpec *corev1.PodSpec, serviceName string) error {
	secretVolumeSpec := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: serviceName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: serviceName,
					},
				},
			},
		},
	}
	secretContainerSpec := corev1.Container{
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      serviceName,
				ReadOnly:  false,
				MountPath: lokiServerGRPCTLSDir(),
			},
		},
		Args: []string{
			fmt.Sprintf("-server.grpc-tls-ca-path=%s", signingCAPath()),
			fmt.Sprintf("-server.grpc-tls-cert-path=%s", lokiServerGRPCTLSCert()),
			fmt.Sprintf("-server.grpc-tls-key-path=%s", lokiServerGRPCTLSKey()),
			"-server.grpc-tls-client-auth=RequireAndVerifyClientCert",
		},
	}

	if err := mergo.Merge(podSpec, secretVolumeSpec, mergo.WithAppendSlice); err != nil {
		return kverrors.Wrap(err, "failed to merge volumes")
	}

	if err := mergo.Merge(&podSpec.Containers[0], secretContainerSpec, mergo.WithAppendSlice); err != nil {
		return kverrors.Wrap(err, "failed to merge container")
	}

	return nil
}

func configureHTTPServicePKI(podSpec *corev1.PodSpec, serviceName, minTLSVersion, tlsCipherSuites string) error {
	secretVolumeSpec := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: serviceName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: serviceName,
					},
				},
			},
		},
	}

	secretContainerSpec := corev1.Container{
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      serviceName,
				ReadOnly:  false,
				MountPath: lokiServerHTTPTLSDir(),
			},
		},
		Args: []string{
			// Expose ready handler through internal server without requiring mTLS
			"-internal-server.enable=true",
			"-internal-server.http-listen-address=",
			fmt.Sprintf("-internal-server.http-tls-min-version=%s", minTLSVersion),
			fmt.Sprintf("-internal-server.http-tls-cipher-suites=%s", tlsCipherSuites),
			fmt.Sprintf("-internal-server.http-tls-cert-path=%s", lokiServerHTTPTLSCert()),
			fmt.Sprintf("-internal-server.http-tls-key-path=%s", lokiServerHTTPTLSKey()),
			// Require mTLS for any other handler
			fmt.Sprintf("-server.http-tls-ca-path=%s", signingCAPath()),
			fmt.Sprintf("-server.http-tls-cert-path=%s", lokiServerHTTPTLSCert()),
			fmt.Sprintf("-server.http-tls-key-path=%s", lokiServerHTTPTLSKey()),
			"-server.http-tls-client-auth=RequireAndVerifyClientCert",
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          lokiInternalHTTPPortName,
				ContainerPort: internalHTTPPort,
				Protocol:      protocolTCP,
			},
		},
	}

	uriSchemeContainerSpec := corev1.Container{
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Scheme: corev1.URISchemeHTTPS,
					Port:   intstr.FromInt(internalHTTPPort),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Scheme: corev1.URISchemeHTTPS,
					Port:   intstr.FromInt(internalHTTPPort),
				},
			},
		},
	}

	if err := mergo.Merge(podSpec, secretVolumeSpec, mergo.WithAppendSlice); err != nil {
		return kverrors.Wrap(err, "failed to merge volumes")
	}

	if err := mergo.Merge(&podSpec.Containers[0], secretContainerSpec, mergo.WithAppendSlice); err != nil {
		return kverrors.Wrap(err, "failed to merge container")
	}

	if err := mergo.Merge(&podSpec.Containers[0], uriSchemeContainerSpec, mergo.WithOverride); err != nil {
		return kverrors.Wrap(err, "failed to merge container")
	}

	return nil
}
