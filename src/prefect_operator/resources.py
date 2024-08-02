from typing import Any, ClassVar, Iterable, Self

from pydantic import BaseModel, PrivateAttr, ValidationInfo, model_validator


class CustomResource(BaseModel):
    kind: ClassVar[str]
    plural: ClassVar[str]
    singular: ClassVar[str]

    @classmethod
    def concrete_resources(cls) -> Iterable[type["CustomResource"]]:
        if hasattr(cls, "kind"):
            yield cls
        for subclass in cls.__subclasses__():
            yield from subclass.concrete_resources()

    @classmethod
    def definitions(cls) -> Iterable[dict[str, Any]]:
        return [resource.definition() for resource in cls.concrete_resources()]

    @classmethod
    def definition(cls) -> dict[str, Any]:
        return {
            "apiVersion": "apiextensions.k8s.io/v1",
            "kind": "CustomResourceDefinition",
            "metadata": {"name": f"{cls.plural}.prefect.io"},
            "spec": {
                "group": "prefect.io",
                "scope": "Namespaced",
                "names": {
                    "kind": cls.kind,
                    "plural": cls.plural,
                    "singular": cls.singular,
                },
                "versions": [
                    {
                        "name": "v3",
                        "served": True,
                        "storage": True,
                        "schema": {
                            "openAPIV3Schema": {
                                "type": "object",
                                "properties": {
                                    "spec": cls.model_json_schema_inlined(),
                                },
                            }
                        },
                    }
                ],
            },
        }

    @classmethod
    def model_json_schema_inlined(cls) -> dict[str, Any]:
        schema = cls.model_json_schema()
        definitions = schema.pop("$defs") or {}

        def resolve_refs(obj: Any):
            if isinstance(obj, dict):
                if "$ref" in obj:
                    ref = obj["$ref"]
                    if isinstance(ref, str) and ref.startswith("#/$defs/"):
                        del obj["$ref"]
                        obj.update(definitions[ref.split("/")[-1]])

                for v in obj.values():
                    resolve_refs(v)

            if isinstance(obj, list):
                for v in obj:
                    resolve_refs(v)

        def collapse_optionals(obj: Any):
            if isinstance(obj, dict):
                if (
                    "anyOf" in obj
                    and len(obj["anyOf"]) == 2
                    and obj["anyOf"][0]["type"] == "object"
                    and obj["anyOf"][1]["type"] == "null"
                ):
                    any_of = obj.pop("anyOf")
                    obj.update(any_of[0])

                for v in obj.values():
                    collapse_optionals(v)

            if isinstance(obj, list):
                for v in obj:
                    collapse_optionals(v)

        resolve_refs(definitions)
        resolve_refs(schema)
        collapse_optionals(schema)

        return schema


class NamedResource(BaseModel):
    _name: str = PrivateAttr()

    @property
    def name(self) -> str:
        return self._name

    _namespace: str = PrivateAttr()

    @property
    def namespace(self) -> str:
        return self._namespace

    @model_validator(mode="after")
    def set_name_and_namespace(self, validation_info: ValidationInfo) -> Self:
        self._name = validation_info.context["name"]
        self._namespace = validation_info.context["namespace"]
        return self