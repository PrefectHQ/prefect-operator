# API Reference

Packages:

- [prefect.io/v1](#prefectiov1)

# prefect.io/v1

Resource Types:

- [PrefectDeployment](#prefectdeployment)




## PrefectDeployment
<sup><sup>[↩ Parent](#prefectiov1 )</sup></sup>






PrefectDeployment is the Schema for the prefectdeployments API

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>prefect.io/v1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>PrefectDeployment</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspec">spec</a></b></td>
        <td>object</td>
        <td>
          PrefectDeploymentSpec defines the desired state of a PrefectDeployment<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentstatus">status</a></b></td>
        <td>object</td>
        <td>
          PrefectDeploymentStatus defines the observed state of PrefectDeployment<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec
<sup><sup>[↩ Parent](#prefectdeployment)</sup></sup>



PrefectDeploymentSpec defines the desired state of a PrefectDeployment

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#prefectdeploymentspecdeployment">deployment</a></b></td>
        <td>object</td>
        <td>
          Deployment configuration defining the Prefect deployment<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecserver">server</a></b></td>
        <td>object</td>
        <td>
          Server configuration for connecting to Prefect API<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecworkpool">workPool</a></b></td>
        <td>object</td>
        <td>
          WorkPool configuration specifying where the deployment should run<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.deployment
<sup><sup>[↩ Parent](#prefectdeploymentspec)</sup></sup>



Deployment configuration defining the Prefect deployment

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>entrypoint</b></td>
        <td>string</td>
        <td>
          Entrypoint is the entrypoint for the flow (e.g., "my_code.py:my_function")<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>concurrencyLimit</b></td>
        <td>integer</td>
        <td>
          ConcurrencyLimit limits concurrent runs of this deployment<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>description</b></td>
        <td>string</td>
        <td>
          Description is a human-readable description of the deployment<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>enforceParameterSchema</b></td>
        <td>boolean</td>
        <td>
          EnforceParameterSchema determines if parameter schema should be enforced<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecdeploymentglobalconcurrencylimit">globalConcurrencyLimit</a></b></td>
        <td>object</td>
        <td>
          GlobalConcurrencyLimit references a global concurrency limit<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>jobVariables</b></td>
        <td>object</td>
        <td>
          JobVariables are variables passed to the infrastructure<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>
          Labels are key-value pairs for additional metadata<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>parameterOpenApiSchema</b></td>
        <td>object</td>
        <td>
          ParameterOpenApiSchema defines the OpenAPI schema for flow parameters<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>parameters</b></td>
        <td>object</td>
        <td>
          Parameters are default parameters for flow runs<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>path</b></td>
        <td>string</td>
        <td>
          Path is the path to the flow code<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>paused</b></td>
        <td>boolean</td>
        <td>
          Paused indicates if the deployment is paused<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>pullSteps</b></td>
        <td>[]object</td>
        <td>
          PullSteps defines steps to retrieve the flow code<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecdeploymentschedulesindex">schedules</a></b></td>
        <td>[]object</td>
        <td>
          Schedules defines when the deployment should run<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>tags</b></td>
        <td>[]string</td>
        <td>
          Tags are labels for organizing and filtering deployments<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecdeploymentversioninfo">versionInfo</a></b></td>
        <td>object</td>
        <td>
          VersionInfo describes the deployment version<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.deployment.globalConcurrencyLimit
<sup><sup>[↩ Parent](#prefectdeploymentspecdeployment)</sup></sup>



GlobalConcurrencyLimit references a global concurrency limit

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the global concurrency limit<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>active</b></td>
        <td>boolean</td>
        <td>
          Active indicates if the limit is active<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>collisionStrategy</b></td>
        <td>string</td>
        <td>
          CollisionStrategy defines behavior when limit is exceeded<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>limit</b></td>
        <td>integer</td>
        <td>
          Limit is the concurrency limit value<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>slotDecayPerSecond</b></td>
        <td>string</td>
        <td>
          SlotDecayPerSecond defines how quickly slots are released<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.deployment.schedules[index]
<sup><sup>[↩ Parent](#prefectdeploymentspecdeployment)</sup></sup>



PrefectSchedule defines a schedule for the deployment

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#prefectdeploymentspecdeploymentschedulesindexschedule">schedule</a></b></td>
        <td>object</td>
        <td>
          Schedule defines the schedule configuration<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>slug</b></td>
        <td>string</td>
        <td>
          Slug is a unique identifier for the schedule<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.deployment.schedules[index].schedule
<sup><sup>[↩ Parent](#prefectdeploymentspecdeploymentschedulesindex)</sup></sup>



Schedule defines the schedule configuration

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>active</b></td>
        <td>boolean</td>
        <td>
          Active indicates if the schedule is active<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>anchorDate</b></td>
        <td>string</td>
        <td>
          AnchorDate is the anchor date for the schedule<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>interval</b></td>
        <td>integer</td>
        <td>
          Interval is the schedule interval in seconds<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>maxScheduledRuns</b></td>
        <td>integer</td>
        <td>
          MaxScheduledRuns limits the number of scheduled runs<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>timezone</b></td>
        <td>string</td>
        <td>
          Timezone for the schedule<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.deployment.versionInfo
<sup><sup>[↩ Parent](#prefectdeploymentspecdeployment)</sup></sup>



VersionInfo describes the deployment version

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          Type is the version type (e.g., "git")<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>
          Version is the version string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.server
<sup><sup>[↩ Parent](#prefectdeploymentspec)</sup></sup>



Server configuration for connecting to Prefect API

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>accountId</b></td>
        <td>string</td>
        <td>
          AccountID is the ID of the account to use to connect to Prefect Cloud<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecserverapikey">apiKey</a></b></td>
        <td>object</td>
        <td>
          APIKey is the API key to use to connect to a remote Prefect Server<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the in-cluster Prefect Server in the given namespace<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace is the namespace where the in-cluster Prefect Server is running<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>remoteApiUrl</b></td>
        <td>string</td>
        <td>
          RemoteAPIURL is the API URL for the remote Prefect Server. Set if using with an external Prefect Server or Prefect Cloud<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>workspaceId</b></td>
        <td>string</td>
        <td>
          WorkspaceID is the ID of the workspace to use to connect to Prefect Cloud<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.server.apiKey
<sup><sup>[↩ Parent](#prefectdeploymentspecserver)</sup></sup>



APIKey is the API key to use to connect to a remote Prefect Server

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value is the literal value of the API key<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecserverapikeyvaluefrom">valueFrom</a></b></td>
        <td>object</td>
        <td>
          ValueFrom is a reference to a secret containing the API key<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.server.apiKey.valueFrom
<sup><sup>[↩ Parent](#prefectdeploymentspecserverapikey)</sup></sup>



ValueFrom is a reference to a secret containing the API key

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#prefectdeploymentspecserverapikeyvaluefromconfigmapkeyref">configMapKeyRef</a></b></td>
        <td>object</td>
        <td>
          Selects a key of a ConfigMap.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecserverapikeyvaluefromfieldref">fieldRef</a></b></td>
        <td>object</td>
        <td>
          Selects a field of the pod: supports metadata.name, metadata.namespace, `metadata.labels['<KEY>']`, `metadata.annotations['<KEY>']`,
spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecserverapikeyvaluefromfilekeyref">fileKeyRef</a></b></td>
        <td>object</td>
        <td>
          FileKeyRef selects a key of the env file.
Requires the EnvFiles feature gate to be enabled.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecserverapikeyvaluefromresourcefieldref">resourceFieldRef</a></b></td>
        <td>object</td>
        <td>
          Selects a resource of the container: only resources limits and requests
(limits.cpu, limits.memory, limits.ephemeral-storage, requests.cpu, requests.memory and requests.ephemeral-storage) are currently supported.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentspecserverapikeyvaluefromsecretkeyref">secretKeyRef</a></b></td>
        <td>object</td>
        <td>
          Selects a key of a secret in the pod's namespace<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.server.apiKey.valueFrom.configMapKeyRef
<sup><sup>[↩ Parent](#prefectdeploymentspecserverapikeyvaluefrom)</sup></sup>



Selects a key of a ConfigMap.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          The key to select.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the referent.
This field is effectively required, but due to backwards compatibility is
allowed to be empty. Instances of this type with an empty value here are
almost certainly wrong.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names<br/>
          <br/>
            <i>Default</i>: <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>optional</b></td>
        <td>boolean</td>
        <td>
          Specify whether the ConfigMap or its key must be defined<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.server.apiKey.valueFrom.fieldRef
<sup><sup>[↩ Parent](#prefectdeploymentspecserverapikeyvaluefrom)</sup></sup>



Selects a field of the pod: supports metadata.name, metadata.namespace, `metadata.labels['<KEY>']`, `metadata.annotations['<KEY>']`,
spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>fieldPath</b></td>
        <td>string</td>
        <td>
          Path of the field to select in the specified API version.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiVersion</b></td>
        <td>string</td>
        <td>
          Version of the schema the FieldPath is written in terms of, defaults to "v1".<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.server.apiKey.valueFrom.fileKeyRef
<sup><sup>[↩ Parent](#prefectdeploymentspecserverapikeyvaluefrom)</sup></sup>



FileKeyRef selects a key of the env file.
Requires the EnvFiles feature gate to be enabled.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          The key within the env file. An invalid key will prevent the pod from starting.
The keys defined within a source may consist of any printable ASCII characters except '='.
During Alpha stage of the EnvFiles feature gate, the key size is limited to 128 characters.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>path</b></td>
        <td>string</td>
        <td>
          The path within the volume from which to select the file.
Must be relative and may not contain the '..' path or start with '..'.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>volumeName</b></td>
        <td>string</td>
        <td>
          The name of the volume mount containing the env file.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>optional</b></td>
        <td>boolean</td>
        <td>
          Specify whether the file or its key must be defined. If the file or key
does not exist, then the env var is not published.
If optional is set to true and the specified key does not exist,
the environment variable will not be set in the Pod's containers.

If optional is set to false and the specified key does not exist,
an error will be returned during Pod creation.<br/>
          <br/>
            <i>Default</i>: false<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.server.apiKey.valueFrom.resourceFieldRef
<sup><sup>[↩ Parent](#prefectdeploymentspecserverapikeyvaluefrom)</sup></sup>



Selects a resource of the container: only resources limits and requests
(limits.cpu, limits.memory, limits.ephemeral-storage, requests.cpu, requests.memory and requests.ephemeral-storage) are currently supported.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>resource</b></td>
        <td>string</td>
        <td>
          Required: resource to select<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>containerName</b></td>
        <td>string</td>
        <td>
          Container name: required for volumes, optional for env vars<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>divisor</b></td>
        <td>int or string</td>
        <td>
          Specifies the output format of the exposed resources, defaults to "1"<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.server.apiKey.valueFrom.secretKeyRef
<sup><sup>[↩ Parent](#prefectdeploymentspecserverapikeyvaluefrom)</sup></sup>



Selects a key of a secret in the pod's namespace

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          The key of the secret to select from.  Must be a valid secret key.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the referent.
This field is effectively required, but due to backwards compatibility is
allowed to be empty. Instances of this type with an empty value here are
almost certainly wrong.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names<br/>
          <br/>
            <i>Default</i>: <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>optional</b></td>
        <td>boolean</td>
        <td>
          Specify whether the Secret or its key must be defined<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.spec.workPool
<sup><sup>[↩ Parent](#prefectdeploymentspec)</sup></sup>



WorkPool configuration specifying where the deployment should run

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the work pool<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace is the namespace containing the work pool<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>workQueue</b></td>
        <td>string</td>
        <td>
          WorkQueue is the specific work queue within the work pool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.status
<sup><sup>[↩ Parent](#prefectdeployment)</sup></sup>



PrefectDeploymentStatus defines the observed state of PrefectDeployment

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>ready</b></td>
        <td>boolean</td>
        <td>
          Ready indicates that the deployment exists and is configured correctly<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectdeploymentstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions store the status conditions of the PrefectDeployment instances<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>flowId</b></td>
        <td>string</td>
        <td>
          FlowId is the flow ID from Prefect<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          Id is the deployment ID from Prefect<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastSyncTime</b></td>
        <td>string</td>
        <td>
          LastSyncTime is the last time the deployment was synced with Prefect<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration tracks the last processed generation<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>specHash</b></td>
        <td>string</td>
        <td>
          SpecHash tracks changes to the spec to minimize API calls<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectDeployment.status.conditions[index]
<sup><sup>[↩ Parent](#prefectdeploymentstatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>
