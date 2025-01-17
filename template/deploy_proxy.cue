parameters: {
	name: *"stellaris-proxy" | string
	namespace: *"stellaris-system" | string
	replicas: *1 | int
	image: string
	coreAddr: string
	clusterRole: string
}

outputs: serviceAccount: {
	apiVersion: "v1"
	kind: "ServiceAccount"
	metadata: {
		name: parameters.name
		namespace: parameters.namespace
	}
}

outputs: clusterRoleBinding: {
	apiVersion: "rbac.authorization.k8s.io/v1"
	kind: "ClusterRoleBinding"
	metadata: name: parameters.name
	subjects: [{
		kind: "ServiceAccount"
		name: parameters.name
		namespace: parameters.namespace
	}]
	roleRef: {
		kind: "ClusterRole"
		name: parameters.clusterRole
		apiGroup: "rbac.authorization.k8s.io"
	}
}

outputs: deployment: {
	apiVersion: "apps/v1"
	kind: "Deployment"
	metadata: {
		name: parameters.name
		namespace: parameters.namespace
		labels: app: parameters.name
	}
	spec: {
		replicas: parameters.replicas
		selector: matchLabels: app: parameters.name
		template: {
			metadata: labels: app: parameters.name
			spec: {
				serviceAccountName: parameters.name
				containers: [{
					name: "stellaris-proxy"
					image: parameters.image
					args: [
						"--core-address=" + parameters.coreAddr,
						"--cluster-name=" + context.clusterName,
						"--addon-path=/cfg/addons.yaml",
					]
					volumeMounts: [{
						mountPath: "/cfg"
						name: "proxy-configuration"
					}]
				}]
				volumes: [{
					configMap: {
						name: parameters.name
					}
					name: "proxy-configuration"
				}]
			}
		}
	}
}