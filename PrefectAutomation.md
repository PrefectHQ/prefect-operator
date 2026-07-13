# API Reference

Packages:

- [prefect.io/v1](#prefectiov1)

# prefect.io/v1

Resource Types:

- [PrefectAutomation](#prefectautomation)




## PrefectAutomation
<sup><sup>[↩ Parent](#prefectiov1 )</sup></sup>






PrefectAutomation is the Schema for the prefectautomations API

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
      <td>PrefectAutomation</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspec">spec</a></b></td>
        <td>object</td>
        <td>
          PrefectAutomationSpec defines the desired state of a PrefectAutomation.
It mirrors the options of the Prefect Terraform provider's prefect_automation
resource so automations can be managed declaratively via the operator.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationstatus">status</a></b></td>
        <td>object</td>
        <td>
          PrefectAutomationStatus defines the observed state of a PrefectAutomation.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec
<sup><sup>[↩ Parent](#prefectautomation)</sup></sup>



PrefectAutomationSpec defines the desired state of a PrefectAutomation.
It mirrors the options of the Prefect Terraform provider's prefect_automation
resource so automations can be managed declaratively via the operator.

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
          Name of the automation<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspecserver">server</a></b></td>
        <td>object</td>
        <td>
          Server configuration for connecting to the Prefect API<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspectrigger">trigger</a></b></td>
        <td>object</td>
        <td>
          Trigger is the criteria for which events this automation covers.
Exactly one of event, metric, compound, or sequence must be set.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspecactionsindex">actions</a></b></td>
        <td>[]object</td>
        <td>
          Actions to perform when the automation is triggered<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspecactionsonresolveindex">actionsOnResolve</a></b></td>
        <td>[]object</td>
        <td>
          ActionsOnResolve are actions executed when the trigger resolves<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspecactionsontriggerindex">actionsOnTrigger</a></b></td>
        <td>[]object</td>
        <td>
          ActionsOnTrigger are actions executed when the trigger fires<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>description</b></td>
        <td>string</td>
        <td>
          Description is human-readable documentation for the automation<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          Enabled activates or deactivates the automation<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>interval</b></td>
        <td>string</td>
        <td>
          Interval is how often to re-check this automation against the Prefect API
to correct out-of-band drift (edits or deletes made directly in Prefect).
Defaults to the operator's --default-resync-interval when unset. Values
below 10s are clamped.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.server
<sup><sup>[↩ Parent](#prefectautomationspec)</sup></sup>



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
        <td><b><a href="#prefectautomationspecserverapikey">apiKey</a></b></td>
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


### PrefectAutomation.spec.server.apiKey
<sup><sup>[↩ Parent](#prefectautomationspecserver)</sup></sup>



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
        <td><b><a href="#prefectautomationspecserverapikeyvaluefrom">valueFrom</a></b></td>
        <td>object</td>
        <td>
          ValueFrom is a reference to a secret containing the API key<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.server.apiKey.valueFrom
<sup><sup>[↩ Parent](#prefectautomationspecserverapikey)</sup></sup>



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
        <td><b><a href="#prefectautomationspecserverapikeyvaluefromconfigmapkeyref">configMapKeyRef</a></b></td>
        <td>object</td>
        <td>
          Selects a key of a ConfigMap.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspecserverapikeyvaluefromfieldref">fieldRef</a></b></td>
        <td>object</td>
        <td>
          Selects a field of the pod: supports metadata.name, metadata.namespace, `metadata.labels['<KEY>']`, `metadata.annotations['<KEY>']`,
spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspecserverapikeyvaluefromfilekeyref">fileKeyRef</a></b></td>
        <td>object</td>
        <td>
          FileKeyRef selects a key of the env file.
Requires the EnvFiles feature gate to be enabled.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspecserverapikeyvaluefromresourcefieldref">resourceFieldRef</a></b></td>
        <td>object</td>
        <td>
          Selects a resource of the container: only resources limits and requests
(limits.cpu, limits.memory, limits.ephemeral-storage, requests.cpu, requests.memory and requests.ephemeral-storage) are currently supported.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspecserverapikeyvaluefromsecretkeyref">secretKeyRef</a></b></td>
        <td>object</td>
        <td>
          Selects a key of a secret in the pod's namespace<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.server.apiKey.valueFrom.configMapKeyRef
<sup><sup>[↩ Parent](#prefectautomationspecserverapikeyvaluefrom)</sup></sup>



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


### PrefectAutomation.spec.server.apiKey.valueFrom.fieldRef
<sup><sup>[↩ Parent](#prefectautomationspecserverapikeyvaluefrom)</sup></sup>



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


### PrefectAutomation.spec.server.apiKey.valueFrom.fileKeyRef
<sup><sup>[↩ Parent](#prefectautomationspecserverapikeyvaluefrom)</sup></sup>



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


### PrefectAutomation.spec.server.apiKey.valueFrom.resourceFieldRef
<sup><sup>[↩ Parent](#prefectautomationspecserverapikeyvaluefrom)</sup></sup>



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


### PrefectAutomation.spec.server.apiKey.valueFrom.secretKeyRef
<sup><sup>[↩ Parent](#prefectautomationspecserverapikeyvaluefrom)</sup></sup>



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


### PrefectAutomation.spec.trigger
<sup><sup>[↩ Parent](#prefectautomationspec)</sup></sup>



Trigger is the criteria for which events this automation covers.
Exactly one of event, metric, compound, or sequence must be set.

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
        <td><b><a href="#prefectautomationspectriggercompound">compound</a></b></td>
        <td>object</td>
        <td>
          Compound trigger fires when a number of child triggers fire within a window<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspectriggerevent">event</a></b></td>
        <td>object</td>
        <td>
          Event trigger reacts to (or waits for the absence of) events<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspectriggermetric">metric</a></b></td>
        <td>object</td>
        <td>
          Metric trigger fires when a metric crosses a threshold<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspectriggersequence">sequence</a></b></td>
        <td>object</td>
        <td>
          Sequence trigger fires when child triggers fire in order within a window<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.compound
<sup><sup>[↩ Parent](#prefectautomationspectrigger)</sup></sup>



Compound trigger fires when a number of child triggers fire within a window

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
        <td><b>require</b></td>
        <td>string</td>
        <td>
          Require is how many child triggers must fire: "any", "all", or a number<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspectriggercompoundtriggersindex">triggers</a></b></td>
        <td>[]object</td>
        <td>
          Triggers are the child triggers (event or metric)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>within</b></td>
        <td>integer</td>
        <td>
          Within is the time window in seconds<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.compound.triggers[index]
<sup><sup>[↩ Parent](#prefectautomationspectriggercompound)</sup></sup>



PrefectChildTrigger is a trigger nested inside a compound or sequence trigger.
Prefect only allows event or metric triggers as children (not further
compound/sequence nesting), which also keeps the CRD schema non-recursive.
Exactly one of event or metric must be set.

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
        <td><b><a href="#prefectautomationspectriggercompoundtriggersindexevent">event</a></b></td>
        <td>object</td>
        <td>
          Event trigger<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspectriggercompoundtriggersindexmetric">metric</a></b></td>
        <td>object</td>
        <td>
          Metric trigger<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.compound.triggers[index].event
<sup><sup>[↩ Parent](#prefectautomationspectriggercompoundtriggersindex)</sup></sup>



Event trigger

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
        <td><b>posture</b></td>
        <td>enum</td>
        <td>
          Posture is the trigger posture.<br/>
          <br/>
            <i>Enum</i>: Reactive, Proactive<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>after</b></td>
        <td>[]string</td>
        <td>
          After is the set of event names that must precede the expected events<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>expect</b></td>
        <td>[]string</td>
        <td>
          Expect is the set of event names the trigger expects to see<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>forEach</b></td>
        <td>[]string</td>
        <td>
          ForEach groups events by the given resource fields<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>match</b></td>
        <td>object</td>
        <td>
          Match is a JSON object matching resources by label<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchRelated</b></td>
        <td>object</td>
        <td>
          MatchRelated is a JSON object matching related resources by label<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>threshold</b></td>
        <td>integer</td>
        <td>
          Threshold is the number of events required to fire the trigger<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>within</b></td>
        <td>integer</td>
        <td>
          Within is the time window in seconds<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.compound.triggers[index].metric
<sup><sup>[↩ Parent](#prefectautomationspectriggercompoundtriggersindex)</sup></sup>



Metric trigger

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
        <td><b><a href="#prefectautomationspectriggercompoundtriggersindexmetricmetric">metric</a></b></td>
        <td>object</td>
        <td>
          Metric is the metric query definition<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>match</b></td>
        <td>object</td>
        <td>
          Match is a JSON object matching resources by label<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchRelated</b></td>
        <td>object</td>
        <td>
          MatchRelated is a JSON object matching related resources by label<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.compound.triggers[index].metric.metric
<sup><sup>[↩ Parent](#prefectautomationspectriggercompoundtriggersindexmetric)</sup></sup>



Metric is the metric query definition

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
        <td><b>firingFor</b></td>
        <td>integer</td>
        <td>
          FiringFor is the duration in seconds the query must breach/resolve continuously<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the metric to query<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>enum</td>
        <td>
          Operator is the comparative operator used to evaluate the query result<br/>
          <br/>
            <i>Enum</i>: <, <=, >, >=<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>range</b></td>
        <td>integer</td>
        <td>
          Range is the lookback duration in seconds for the metric query<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>threshold</b></td>
        <td>int or string</td>
        <td>
          Threshold is the value the metric is compared against (may be fractional)<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.event
<sup><sup>[↩ Parent](#prefectautomationspectrigger)</sup></sup>



Event trigger reacts to (or waits for the absence of) events

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
        <td><b>posture</b></td>
        <td>enum</td>
        <td>
          Posture is the trigger posture.<br/>
          <br/>
            <i>Enum</i>: Reactive, Proactive<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>after</b></td>
        <td>[]string</td>
        <td>
          After is the set of event names that must precede the expected events<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>expect</b></td>
        <td>[]string</td>
        <td>
          Expect is the set of event names the trigger expects to see<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>forEach</b></td>
        <td>[]string</td>
        <td>
          ForEach groups events by the given resource fields<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>match</b></td>
        <td>object</td>
        <td>
          Match is a JSON object matching resources by label<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchRelated</b></td>
        <td>object</td>
        <td>
          MatchRelated is a JSON object matching related resources by label<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>threshold</b></td>
        <td>integer</td>
        <td>
          Threshold is the number of events required to fire the trigger<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>within</b></td>
        <td>integer</td>
        <td>
          Within is the time window in seconds<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.metric
<sup><sup>[↩ Parent](#prefectautomationspectrigger)</sup></sup>



Metric trigger fires when a metric crosses a threshold

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
        <td><b><a href="#prefectautomationspectriggermetricmetric">metric</a></b></td>
        <td>object</td>
        <td>
          Metric is the metric query definition<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>match</b></td>
        <td>object</td>
        <td>
          Match is a JSON object matching resources by label<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchRelated</b></td>
        <td>object</td>
        <td>
          MatchRelated is a JSON object matching related resources by label<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.metric.metric
<sup><sup>[↩ Parent](#prefectautomationspectriggermetric)</sup></sup>



Metric is the metric query definition

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
        <td><b>firingFor</b></td>
        <td>integer</td>
        <td>
          FiringFor is the duration in seconds the query must breach/resolve continuously<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the metric to query<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>enum</td>
        <td>
          Operator is the comparative operator used to evaluate the query result<br/>
          <br/>
            <i>Enum</i>: <, <=, >, >=<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>range</b></td>
        <td>integer</td>
        <td>
          Range is the lookback duration in seconds for the metric query<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>threshold</b></td>
        <td>int or string</td>
        <td>
          Threshold is the value the metric is compared against (may be fractional)<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.sequence
<sup><sup>[↩ Parent](#prefectautomationspectrigger)</sup></sup>



Sequence trigger fires when child triggers fire in order within a window

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
        <td><b><a href="#prefectautomationspectriggersequencetriggersindex">triggers</a></b></td>
        <td>[]object</td>
        <td>
          Triggers are the child triggers, which must fire in order<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>within</b></td>
        <td>integer</td>
        <td>
          Within is the time window in seconds<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.sequence.triggers[index]
<sup><sup>[↩ Parent](#prefectautomationspectriggersequence)</sup></sup>



PrefectChildTrigger is a trigger nested inside a compound or sequence trigger.
Prefect only allows event or metric triggers as children (not further
compound/sequence nesting), which also keeps the CRD schema non-recursive.
Exactly one of event or metric must be set.

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
        <td><b><a href="#prefectautomationspectriggersequencetriggersindexevent">event</a></b></td>
        <td>object</td>
        <td>
          Event trigger<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#prefectautomationspectriggersequencetriggersindexmetric">metric</a></b></td>
        <td>object</td>
        <td>
          Metric trigger<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.sequence.triggers[index].event
<sup><sup>[↩ Parent](#prefectautomationspectriggersequencetriggersindex)</sup></sup>



Event trigger

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
        <td><b>posture</b></td>
        <td>enum</td>
        <td>
          Posture is the trigger posture.<br/>
          <br/>
            <i>Enum</i>: Reactive, Proactive<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>after</b></td>
        <td>[]string</td>
        <td>
          After is the set of event names that must precede the expected events<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>expect</b></td>
        <td>[]string</td>
        <td>
          Expect is the set of event names the trigger expects to see<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>forEach</b></td>
        <td>[]string</td>
        <td>
          ForEach groups events by the given resource fields<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>match</b></td>
        <td>object</td>
        <td>
          Match is a JSON object matching resources by label<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchRelated</b></td>
        <td>object</td>
        <td>
          MatchRelated is a JSON object matching related resources by label<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>threshold</b></td>
        <td>integer</td>
        <td>
          Threshold is the number of events required to fire the trigger<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>within</b></td>
        <td>integer</td>
        <td>
          Within is the time window in seconds<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.sequence.triggers[index].metric
<sup><sup>[↩ Parent](#prefectautomationspectriggersequencetriggersindex)</sup></sup>



Metric trigger

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
        <td><b><a href="#prefectautomationspectriggersequencetriggersindexmetricmetric">metric</a></b></td>
        <td>object</td>
        <td>
          Metric is the metric query definition<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>match</b></td>
        <td>object</td>
        <td>
          Match is a JSON object matching resources by label<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchRelated</b></td>
        <td>object</td>
        <td>
          MatchRelated is a JSON object matching related resources by label<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.trigger.sequence.triggers[index].metric.metric
<sup><sup>[↩ Parent](#prefectautomationspectriggersequencetriggersindexmetric)</sup></sup>



Metric is the metric query definition

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
        <td><b>firingFor</b></td>
        <td>integer</td>
        <td>
          FiringFor is the duration in seconds the query must breach/resolve continuously<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the metric to query<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>enum</td>
        <td>
          Operator is the comparative operator used to evaluate the query result<br/>
          <br/>
            <i>Enum</i>: <, <=, >, >=<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>range</b></td>
        <td>integer</td>
        <td>
          Range is the lookback duration in seconds for the metric query<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>threshold</b></td>
        <td>int or string</td>
        <td>
          Threshold is the value the metric is compared against (may be fractional)<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.actions[index]
<sup><sup>[↩ Parent](#prefectautomationspec)</sup></sup>



PrefectAutomationAction matches Prefect's action union. `type` selects the
action; the remaining fields apply to the relevant action types.

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
        <td>enum</td>
        <td>
          Type is the action to perform.<br/>
          <br/>
            <i>Enum</i>: do-nothing, run-deployment, pause-deployment, resume-deployment, cancel-flow-run, change-flow-run-state, suspend-flow-run, resume-flow-run, pause-work-queue, resume-work-queue, pause-work-pool, resume-work-pool, pause-automation, resume-automation, send-notification, call-webhook, declare-incident, pause-schedule-for-flow-run, resume-schedule-for-flow-run<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>automationId</b></td>
        <td>string</td>
        <td>
          AutomationID targets an automation (pause/resume-automation)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>blockDocumentId</b></td>
        <td>string</td>
        <td>
          BlockDocumentID targets a block document (send-notification/call-webhook)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>body</b></td>
        <td>string</td>
        <td>
          Body is the notification body<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>deploymentId</b></td>
        <td>string</td>
        <td>
          DeploymentID targets a deployment (run/pause/resume-deployment) by its
Prefect UUID.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>deploymentName</b></td>
        <td>string</td>
        <td>
          DeploymentName targets a deployment (run/pause/resume-deployment) by its
Prefect deployment name; the operator resolves it to a deployment ID at
reconcile time (so automations can reference deployments declaratively,
without knowing the runtime-assigned UUID). Mutually exclusive with
DeploymentID. If the named deployment does not exist yet, reconciliation
requeues until it does.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>jobVariables</b></td>
        <td>object</td>
        <td>
          JobVariables is a JSON object of job variables for run-deployment<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          Message is the state message (change-flow-run-state) or notification body<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the state name for change-flow-run-state<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>parameters</b></td>
        <td>object</td>
        <td>
          Parameters is a JSON object of parameters for run-deployment<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>payload</b></td>
        <td>string</td>
        <td>
          Payload is the webhook payload (call-webhook)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>scheduleId</b></td>
        <td>string</td>
        <td>
          ScheduleID targets a schedule (pause/resume-schedule-for-flow-run)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>source</b></td>
        <td>enum</td>
        <td>
          Source selects whether the target is "selected" (explicit ID) or "inferred"
from the triggering event.<br/>
          <br/>
            <i>Enum</i>: selected, inferred<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>string</td>
        <td>
          State is the target state for change-flow-run-state<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>subject</b></td>
        <td>string</td>
        <td>
          Subject is the notification subject<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>workPoolId</b></td>
        <td>string</td>
        <td>
          WorkPoolID targets a work pool (pause/resume-work-pool)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>workQueueId</b></td>
        <td>string</td>
        <td>
          WorkQueueID targets a work queue (pause/resume-work-queue)<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.actionsOnResolve[index]
<sup><sup>[↩ Parent](#prefectautomationspec)</sup></sup>



PrefectAutomationAction matches Prefect's action union. `type` selects the
action; the remaining fields apply to the relevant action types.

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
        <td>enum</td>
        <td>
          Type is the action to perform.<br/>
          <br/>
            <i>Enum</i>: do-nothing, run-deployment, pause-deployment, resume-deployment, cancel-flow-run, change-flow-run-state, suspend-flow-run, resume-flow-run, pause-work-queue, resume-work-queue, pause-work-pool, resume-work-pool, pause-automation, resume-automation, send-notification, call-webhook, declare-incident, pause-schedule-for-flow-run, resume-schedule-for-flow-run<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>automationId</b></td>
        <td>string</td>
        <td>
          AutomationID targets an automation (pause/resume-automation)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>blockDocumentId</b></td>
        <td>string</td>
        <td>
          BlockDocumentID targets a block document (send-notification/call-webhook)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>body</b></td>
        <td>string</td>
        <td>
          Body is the notification body<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>deploymentId</b></td>
        <td>string</td>
        <td>
          DeploymentID targets a deployment (run/pause/resume-deployment) by its
Prefect UUID.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>deploymentName</b></td>
        <td>string</td>
        <td>
          DeploymentName targets a deployment (run/pause/resume-deployment) by its
Prefect deployment name; the operator resolves it to a deployment ID at
reconcile time (so automations can reference deployments declaratively,
without knowing the runtime-assigned UUID). Mutually exclusive with
DeploymentID. If the named deployment does not exist yet, reconciliation
requeues until it does.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>jobVariables</b></td>
        <td>object</td>
        <td>
          JobVariables is a JSON object of job variables for run-deployment<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          Message is the state message (change-flow-run-state) or notification body<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the state name for change-flow-run-state<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>parameters</b></td>
        <td>object</td>
        <td>
          Parameters is a JSON object of parameters for run-deployment<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>payload</b></td>
        <td>string</td>
        <td>
          Payload is the webhook payload (call-webhook)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>scheduleId</b></td>
        <td>string</td>
        <td>
          ScheduleID targets a schedule (pause/resume-schedule-for-flow-run)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>source</b></td>
        <td>enum</td>
        <td>
          Source selects whether the target is "selected" (explicit ID) or "inferred"
from the triggering event.<br/>
          <br/>
            <i>Enum</i>: selected, inferred<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>string</td>
        <td>
          State is the target state for change-flow-run-state<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>subject</b></td>
        <td>string</td>
        <td>
          Subject is the notification subject<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>workPoolId</b></td>
        <td>string</td>
        <td>
          WorkPoolID targets a work pool (pause/resume-work-pool)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>workQueueId</b></td>
        <td>string</td>
        <td>
          WorkQueueID targets a work queue (pause/resume-work-queue)<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.spec.actionsOnTrigger[index]
<sup><sup>[↩ Parent](#prefectautomationspec)</sup></sup>



PrefectAutomationAction matches Prefect's action union. `type` selects the
action; the remaining fields apply to the relevant action types.

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
        <td>enum</td>
        <td>
          Type is the action to perform.<br/>
          <br/>
            <i>Enum</i>: do-nothing, run-deployment, pause-deployment, resume-deployment, cancel-flow-run, change-flow-run-state, suspend-flow-run, resume-flow-run, pause-work-queue, resume-work-queue, pause-work-pool, resume-work-pool, pause-automation, resume-automation, send-notification, call-webhook, declare-incident, pause-schedule-for-flow-run, resume-schedule-for-flow-run<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>automationId</b></td>
        <td>string</td>
        <td>
          AutomationID targets an automation (pause/resume-automation)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>blockDocumentId</b></td>
        <td>string</td>
        <td>
          BlockDocumentID targets a block document (send-notification/call-webhook)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>body</b></td>
        <td>string</td>
        <td>
          Body is the notification body<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>deploymentId</b></td>
        <td>string</td>
        <td>
          DeploymentID targets a deployment (run/pause/resume-deployment) by its
Prefect UUID.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>deploymentName</b></td>
        <td>string</td>
        <td>
          DeploymentName targets a deployment (run/pause/resume-deployment) by its
Prefect deployment name; the operator resolves it to a deployment ID at
reconcile time (so automations can reference deployments declaratively,
without knowing the runtime-assigned UUID). Mutually exclusive with
DeploymentID. If the named deployment does not exist yet, reconciliation
requeues until it does.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>jobVariables</b></td>
        <td>object</td>
        <td>
          JobVariables is a JSON object of job variables for run-deployment<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          Message is the state message (change-flow-run-state) or notification body<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the state name for change-flow-run-state<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>parameters</b></td>
        <td>object</td>
        <td>
          Parameters is a JSON object of parameters for run-deployment<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>payload</b></td>
        <td>string</td>
        <td>
          Payload is the webhook payload (call-webhook)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>scheduleId</b></td>
        <td>string</td>
        <td>
          ScheduleID targets a schedule (pause/resume-schedule-for-flow-run)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>source</b></td>
        <td>enum</td>
        <td>
          Source selects whether the target is "selected" (explicit ID) or "inferred"
from the triggering event.<br/>
          <br/>
            <i>Enum</i>: selected, inferred<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>string</td>
        <td>
          State is the target state for change-flow-run-state<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>subject</b></td>
        <td>string</td>
        <td>
          Subject is the notification subject<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>workPoolId</b></td>
        <td>string</td>
        <td>
          WorkPoolID targets a work pool (pause/resume-work-pool)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>workQueueId</b></td>
        <td>string</td>
        <td>
          WorkQueueID targets a work queue (pause/resume-work-queue)<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PrefectAutomation.status
<sup><sup>[↩ Parent](#prefectautomation)</sup></sup>



PrefectAutomationStatus defines the observed state of a PrefectAutomation.

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
          Ready indicates that the automation exists and is configured correctly<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#prefectautomationstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions store the status conditions of the PrefectAutomation instances<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          Id is the automation ID from Prefect<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastSyncTime</b></td>
        <td>string</td>
        <td>
          LastSyncTime is the last time the automation was synced with Prefect<br/>
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


### PrefectAutomation.status.conditions[index]
<sup><sup>[↩ Parent](#prefectautomationstatus)</sup></sup>



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
