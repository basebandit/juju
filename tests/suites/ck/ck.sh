# Ensure that Charmed Kubernetes deploys successfully, and that we can
# create storage on the cluster using kubectl after it's deployed.
run_deploy_ck() {
	echo

	local name model_name file overlay_path kube_home storage_path
	name="deploy-ck"
	model_name="${name}"
	file="${TEST_DIR}/${model_name}.log"

	ensure "${model_name}" "${file}"

	overlay_path="./tests/suites/ck/overlay/${BOOTSTRAP_PROVIDER}.yaml"
	# TODO: pin CK test to 1.24/stable for now, remove once 1.25/stable fixed.
	# Issue: `0/5 nodes are available: 5 pod has unbound immediate PersistentVolumeClaims. preemption: 0/5 nodes are available: 5 Preemption is not helpful for scheduling.` But the cluster does have 5 READY worker nodes.
	juju deploy charmed-kubernetes --overlay "${overlay_path}" --trust --channel 1.24/stable

	if ! which "kubectl" >/dev/null 2>&1; then
		sudo snap install kubectl --classic --channel latest/stable
	fi

	wait_for "active" '.applications["kubernetes-control-plane"] | ."application-status".current' 1800
	wait_for "active" '.applications["kubernetes-worker"] | ."application-status".current'

	kube_home="${HOME}/.kube"
	mkdir -p "${kube_home}"
	juju scp kubernetes-control-plane/0:config "${kube_home}/config"

	kubectl cluster-info
	kubectl get ns
	storage_path="./tests/suites/ck/storage/${BOOTSTRAP_PROVIDER}.yaml"
	kubectl create -f "${storage_path}"
	kubectl get sc -o yaml

	# The model teardown could take too long time, so we decided to kill controller to speed up test run time.
	# But this will not give the chance for integrator charm to do proper cleanup:
	# - https://github.com/juju-solutions/charm-aws-integrator/blob/master/lib/charms/layer/aws.py#L616
	# - especially the tag cleanup: https://github.com/juju-solutions/charm-aws-integrator/blob/master/lib/charms/layer/aws.py#L616
	# This will leave the tags created by the integrater charm on subnets forever.
	# And on AWS, the maximum number of tags per resource is 50.
	# Then we will get `Error while granting requests (TagLimitExceeded); check credentials and debug-log` error in next test run.
	# So we purge the subnet tags here in advance as a workaround.
	integrator_app_name=$(cat "$overlay_path" | yq '.applications | keys | .[] | select(.== "*integrator")')
	juju --show-log run-action "$integrator_app_name/leader" --wait=10m purge-subnet-tags
	# juju --show-log run "$integrator_app_name/leader"--wait=10m purge-subnet-tags  # 3.0
}

# Ensure that a CAAS workload (mariadb+mediawiki) deploys successfully,
# and that we can relate the two applications once it has.
run_deploy_caas_workload() {
	echo

	local name k8s_cloud_name model_name file storage controller_name

	name="deploy-caas-workload"
	k8s_cloud_name="k8s-cloud"
	model_name="test-${name}"
	file="${TEST_DIR}/${model_name}.log"

	storage=$(kubectl get sc -o json | jq -r '.items[] | select(.metadata.annotations."storageclass.kubernetes.io/is-default-class"=="true") | .metadata.name')

	controller_name=$(juju controllers --format json | jq -r '.controllers | keys[0]')
	juju add-k8s "${k8s_cloud_name}" --storage "${storage}" --controller "${controller_name}" 2>&1 | OUTPUT "${file}"

	add_model "${model_name}" "${k8s_cloud_name}" "${controller_name}" "${file}"

	juju deploy cs:~juju/mariadb-k8s-3
	juju deploy cs:~juju/mediawiki-k8s-4 --config kubernetes-service-type=loadbalancer
	juju relate mediawiki-k8s:db mariadb-k8s:server

	wait_for "active" '.applications["mariadb-k8s"] | ."application-status".current' 300
	wait_for "active" '.applications["mediawiki-k8s"] | ."application-status".current'

	destroy_model "${model_name}"
}

test_deploy_ck() {
	if [ "$(skip 'test_deploy_ck')" ]; then
		echo "==> TEST SKIPPED: Test Deploy CK"
		return
	fi

	(
		set_verbosity

		cd .. || exit

		run "run_deploy_ck"
		run "run_deploy_caas_workload"
	)
}
