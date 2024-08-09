//go:build !ignore_autogenerated

/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EphemeralConfiguration) DeepCopyInto(out *EphemeralConfiguration) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EphemeralConfiguration.
func (in *EphemeralConfiguration) DeepCopy() *EphemeralConfiguration {
	if in == nil {
		return nil
	}
	out := new(EphemeralConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PostgresConfiguration) DeepCopyInto(out *PostgresConfiguration) {
	*out = *in
	if in.Host != nil {
		in, out := &in.Host, &out.Host
		*out = new(string)
		**out = **in
	}
	if in.HostFrom != nil {
		in, out := &in.HostFrom, &out.HostFrom
		*out = new(corev1.EnvVarSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Port != nil {
		in, out := &in.Port, &out.Port
		*out = new(int)
		**out = **in
	}
	if in.PortFrom != nil {
		in, out := &in.PortFrom, &out.PortFrom
		*out = new(corev1.EnvVarSource)
		(*in).DeepCopyInto(*out)
	}
	if in.User != nil {
		in, out := &in.User, &out.User
		*out = new(string)
		**out = **in
	}
	if in.UserFrom != nil {
		in, out := &in.UserFrom, &out.UserFrom
		*out = new(corev1.EnvVarSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Password != nil {
		in, out := &in.Password, &out.Password
		*out = new(string)
		**out = **in
	}
	if in.PasswordFrom != nil {
		in, out := &in.PasswordFrom, &out.PasswordFrom
		*out = new(corev1.EnvVarSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Database != nil {
		in, out := &in.Database, &out.Database
		*out = new(string)
		**out = **in
	}
	if in.DatabaseFrom != nil {
		in, out := &in.DatabaseFrom, &out.DatabaseFrom
		*out = new(corev1.EnvVarSource)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PostgresConfiguration.
func (in *PostgresConfiguration) DeepCopy() *PostgresConfiguration {
	if in == nil {
		return nil
	}
	out := new(PostgresConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrefectServer) DeepCopyInto(out *PrefectServer) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrefectServer.
func (in *PrefectServer) DeepCopy() *PrefectServer {
	if in == nil {
		return nil
	}
	out := new(PrefectServer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PrefectServer) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrefectServerList) DeepCopyInto(out *PrefectServerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PrefectServer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrefectServerList.
func (in *PrefectServerList) DeepCopy() *PrefectServerList {
	if in == nil {
		return nil
	}
	out := new(PrefectServerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PrefectServerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrefectServerReference) DeepCopyInto(out *PrefectServerReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrefectServerReference.
func (in *PrefectServerReference) DeepCopy() *PrefectServerReference {
	if in == nil {
		return nil
	}
	out := new(PrefectServerReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrefectServerSpec) DeepCopyInto(out *PrefectServerSpec) {
	*out = *in
	if in.Version != nil {
		in, out := &in.Version, &out.Version
		*out = new(string)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.Ephemeral != nil {
		in, out := &in.Ephemeral, &out.Ephemeral
		*out = new(EphemeralConfiguration)
		**out = **in
	}
	if in.SQLite != nil {
		in, out := &in.SQLite, &out.SQLite
		*out = new(SQLiteConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Postgres != nil {
		in, out := &in.Postgres, &out.Postgres
		*out = new(PostgresConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Settings != nil {
		in, out := &in.Settings, &out.Settings
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrefectServerSpec.
func (in *PrefectServerSpec) DeepCopy() *PrefectServerSpec {
	if in == nil {
		return nil
	}
	out := new(PrefectServerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrefectServerStatus) DeepCopyInto(out *PrefectServerStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrefectServerStatus.
func (in *PrefectServerStatus) DeepCopy() *PrefectServerStatus {
	if in == nil {
		return nil
	}
	out := new(PrefectServerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrefectWorkPool) DeepCopyInto(out *PrefectWorkPool) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrefectWorkPool.
func (in *PrefectWorkPool) DeepCopy() *PrefectWorkPool {
	if in == nil {
		return nil
	}
	out := new(PrefectWorkPool)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PrefectWorkPool) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrefectWorkPoolList) DeepCopyInto(out *PrefectWorkPoolList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PrefectWorkPool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrefectWorkPoolList.
func (in *PrefectWorkPoolList) DeepCopy() *PrefectWorkPoolList {
	if in == nil {
		return nil
	}
	out := new(PrefectWorkPoolList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PrefectWorkPoolList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrefectWorkPoolSpec) DeepCopyInto(out *PrefectWorkPoolSpec) {
	*out = *in
	out.Server = in.Server
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrefectWorkPoolSpec.
func (in *PrefectWorkPoolSpec) DeepCopy() *PrefectWorkPoolSpec {
	if in == nil {
		return nil
	}
	out := new(PrefectWorkPoolSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrefectWorkPoolStatus) DeepCopyInto(out *PrefectWorkPoolStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrefectWorkPoolStatus.
func (in *PrefectWorkPoolStatus) DeepCopy() *PrefectWorkPoolStatus {
	if in == nil {
		return nil
	}
	out := new(PrefectWorkPoolStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SQLiteConfiguration) DeepCopyInto(out *SQLiteConfiguration) {
	*out = *in
	out.Size = in.Size.DeepCopy()
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SQLiteConfiguration.
func (in *SQLiteConfiguration) DeepCopy() *SQLiteConfiguration {
	if in == nil {
		return nil
	}
	out := new(SQLiteConfiguration)
	in.DeepCopyInto(out)
	return out
}
