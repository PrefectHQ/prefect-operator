# API Reference

Packages:

- [prefect.io/v1](#prefectiov1)

# prefect.io/v1

Resource Types:

- [PrefectWorkQueue](#prefectworkqueue)




## PrefectWorkQueue
<sup><sup>[↩ Parent](#prefectiov1 )</sup></sup>






PrefectWorkQueue is the Schema for the prefectworkqueues API

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
      <td>PrefectWorkQueue</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectworkqueuespec">spec</a></b></td>
        <td>object</td>
        <td>
          PrefectWorkQueueSpec defines the desired state of a PrefectWorkQueue.
It mirrors the options of the Prefect Terraform provider's prefect_work_queue
resource so work queues can be managed declaratively via the operator.

A queue referenced by a PrefectDeployment's workQueue field is created
implicitly by Prefect with no concurrency limit; declaring it here manages
that limit (and priority) as config. Prefer a work-queue concurrency limit
over a deployment-level one when run ORDER matters: workers pull from a
queue sorted by next scheduled start time, whereas a deployment limit
rejects the transition and re-schedules the run with a fresh timestamp,
which loses the original ordering.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectworkqueuestatus">status</a></b></td>
        <td>object</td>
        <td>
          PrefectWorkQueueStatus defines the observed state of a PrefectWorkQueue.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectWorkQueue.spec
<sup><sup>[↩ Parent](#prefectworkqueue)</sup></sup>



PrefectWorkQueueSpec defines the desired state of a PrefectWorkQueue.
It mirrors the options of the Prefect Terraform provider's prefect_work_queue
resource so work queues can be managed declaratively via the operator.

A queue referenced by a PrefectDeployment's workQueue field is created
implicitly by Prefect with no concurrency limit; declaring it here manages
that limit (and priority) as config. Prefer a work-queue concurrency limit
over a deployment-level one when run ORDER matters: workers pull from a
queue sorted by next scheduled start time, whereas a deployment limit
rejects the transition and re-schedules the run with a fresh timestamp,
which loses the original ordering.

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
          Name of the work queue, as referenced by a deployment's workQueue field.
The queue is managed by (workPoolName, name), never renamed in place:
changing this stops managing the old queue (it is left untouched in
Prefect) and creates — or adopts, if it already exists — a queue under
the new name.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectworkqueuespecserver">server</a></b></td>
        <td>object</td>
        <td>
          Server configuration for connecting to the Prefect API<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>workPoolName</b></td>
        <td>string</td>
        <td>
          WorkPoolName is the work pool this queue belongs to. A queue cannot move
between pools, so this field is immutable.<br/>
          <br/>
            <i>Validations</i>:<li>self == oldSelf: workPoolName is immutable</li>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>concurrencyLimit</b></td>
        <td>integer</td>
        <td>
          ConcurrencyLimit caps how many flow runs this queue may have running at
once. Unset on create leaves the queue unlimited; removing the field
after it has been applied clears the limit in Prefect (the operator
tracks the last-applied field set in status and sends an explicit null).<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>description</b></td>
        <td>string</td>
        <td>
          Description of the queue. Removing the field after it has been applied
clears it in Prefect.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>interval</b></td>
        <td>string</td>
        <td>
          Interval is how often to re-check this work queue against the Prefect API
to correct out-of-band drift (edits or deletes made directly in Prefect).
Defaults to the operator's --default-resync-interval when unset. Values
below 10s are clamped.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>isPaused</b></td>
        <td>boolean</td>
        <td>
          IsPaused stops the queue from serving work when true. Removing the field
after it has been applied unpauses the queue (resets to false, the
create-time default).<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>priority</b></td>
        <td>integer</td>
        <td>
          Priority of this queue within the pool; lower numbers are served first.
Priority is POOL-WIDE state, not per-queue state: Prefect keeps
priorities unique and sequential across the pool, so applying one here
reshuffles the pool's other queues. Two PrefectWorkQueues in the same
pool declaring the same priority is not rejected — Prefect renormalizes
and the last writer wins the slot. Unlike the other optional fields,
priority has no create-time default to restore, so removing it keeps
the last value.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 1<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectWorkQueue.spec.server
<sup><sup>[↩ Parent](#prefectworkqueuespec)</sup></sup>



Server configuration for connecting to the Prefect API

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
        <td><b><a href="#prefectworkqueuespecserverapikey">apiKey</a></b></td>
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


### PrefectWorkQueue.spec.server.apiKey
<sup><sup>[↩ Parent](#prefectworkqueuespecserver)</sup></sup>



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
        <td><b><a href="#prefectworkqueuespecserverapikeyvaluefrom">valueFrom</a></b></td>
        <td>object</td>
        <td>
          ValueFrom is a reference to a secret containing the API key<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectWorkQueue.spec.server.apiKey.valueFrom
<sup><sup>[↩ Parent](#prefectworkqueuespecserverapikey)</sup></sup>



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
        <td><b><a href="#prefectworkqueuespecserverapikeyvaluefromconfigmapkeyref">configMapKeyRef</a></b></td>
        <td>object</td>
        <td>
          Selects a key of a ConfigMap.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectworkqueuespecserverapikeyvaluefromfieldref">fieldRef</a></b></td>
        <td>object</td>
        <td>
          Selects a field of the pod: supports metadata.name, metadata.namespace, `metadata.labels['<KEY>']`, `metadata.annotations['<KEY>']`,
spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectworkqueuespecserverapikeyvaluefromfilekeyref">fileKeyRef</a></b></td>
        <td>object</td>
        <td>
          FileKeyRef selects a key of the env file.
Requires the EnvFiles feature gate to be enabled.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectworkqueuespecserverapikeyvaluefromresourcefieldref">resourceFieldRef</a></b></td>
        <td>object</td>
        <td>
          Selects a resource of the container: only resources limits and requests
(limits.cpu, limits.memory, limits.ephemeral-storage, requests.cpu, requests.memory and requests.ephemeral-storage) are currently supported.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectworkqueuespecserverapikeyvaluefromsecretkeyref">secretKeyRef</a></b></td>
        <td>object</td>
        <td>
          Selects a key of a secret in the pod's namespace<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectWorkQueue.spec.server.apiKey.valueFrom.configMapKeyRef
<sup><sup>[↩ Parent](#prefectworkqueuespecserverapikeyvaluefrom)</sup></sup>



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


### PrefectWorkQueue.spec.server.apiKey.valueFrom.fieldRef
<sup><sup>[↩ Parent](#prefectworkqueuespecserverapikeyvaluefrom)</sup></sup>



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


### PrefectWorkQueue.spec.server.apiKey.valueFrom.fileKeyRef
<sup><sup>[↩ Parent](#prefectworkqueuespecserverapikeyvaluefrom)</sup></sup>



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


### PrefectWorkQueue.spec.server.apiKey.valueFrom.resourceFieldRef
<sup><sup>[↩ Parent](#prefectworkqueuespecserverapikeyvaluefrom)</sup></sup>



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


### PrefectWorkQueue.spec.server.apiKey.valueFrom.secretKeyRef
<sup><sup>[↩ Parent](#prefectworkqueuespecserverapikeyvaluefrom)</sup></sup>



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


### PrefectWorkQueue.status
<sup><sup>[↩ Parent](#prefectworkqueue)</sup></sup>



PrefectWorkQueueStatus defines the observed state of a PrefectWorkQueue.

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
          Ready indicates that the work queue exists and is configured correctly<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>adopted</b></td>
        <td>boolean</td>
        <td>
          Adopted is true when the queue already existed in Prefect the first time
this resource reconciled (e.g. it was created implicitly by a deployment
referencing it). Deleting the resource leaves an adopted queue in place;
only queues this resource created are deleted from Prefect.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>appliedFields</b></td>
        <td>[]string</td>
        <td>
          AppliedFields records which optional spec fields the last successful
sync declared, so a field removed from the spec can be reset to its
create-time default in Prefect instead of silently keeping its old value.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectworkqueuestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions store the status conditions of the PrefectWorkQueue instances<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          Id is the work queue ID from Prefect<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastSyncTime</b></td>
        <td>string</td>
        <td>
          LastSyncTime is the last time the work queue was synced with Prefect<br/>
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


### PrefectWorkQueue.status.conditions[index]
<sup><sup>[↩ Parent](#prefectworkqueuestatus)</sup></sup>



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
