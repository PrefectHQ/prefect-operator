#!/usr/bin/env python3
# /// script
# requires-python = ">=3.9"
# dependencies = [
#     "prefect>=3.4.0",
#     "pyyaml",
#     "typer>=0.9.0",
# ]
# ///
"""
Prefect to Kubernetes CRD Converter

A self-contained PEP 723 script that converts Prefect flows to PrefectDeployment CRDs,
following the same pattern as `prefect deploy` but outputting Kubernetes manifests.

Prerequisites:
- uv (for running the PEP 723 script)
- A Python file with a @flow decorated function

Basic Usage:
    # Output to stdout
    uv run create-prefect-deployment.py flows/my_flow.py:my_flow_function \\
        --work-pool my-pool \\
        --server my-prefect-server \\
        --name my-deployment

    # Pipe to a file
    uv run create-prefect-deployment.py flows/my_flow.py:my_flow_function \\
        --work-pool my-pool \\
        --server my-prefect-server \\
        --name my-deployment > deployment.yaml

Features:
- Automatic parameter schema generation from Python type hints
- No temporary files (unlike `prefect deploy`)
- Kubernetes native CRD output
- Can be piped directly to `kubectl apply -f -`

Example with all options:
    uv run create-prefect-deployment.py flows/my_flow.py:my_flow \\
        --work-pool kubernetes-pool \\
        --server my-prefect-server \\
        --name hello-deployment \\
        --namespace production \\
        --description "Production deployment" \\
        --tag production --tag api \\
        --parameters '{"name": "Kubernetes", "count": 3}' \\
        --job-variables '{"cpu": "500m", "memory": "1Gi"}' \\
        --concurrency-limit 5 \\
        --output deployment.yaml
"""

import sys
import json
from pathlib import Path
from typing import Optional, List, Dict, Any

import yaml
import typer
from prefect import flow, Flow
from prefect.flows import load_flow_from_entrypoint
from prefect.utilities.callables import parameter_schema_from_entrypoint


app = typer.Typer(
    help="Convert Prefect flows to Kubernetes PrefectDeployment CRDs",
    epilog="""
Examples:

  # Basic usage - output to stdout
  uv run create-prefect-deployment.py flows/my_flow.py:my_flow \\
    --work-pool my-pool --server my-server --name my-deployment

  # Pipe to file or kubectl
  uv run create-prefect-deployment.py flows/my_flow.py:my_flow \\
    --work-pool my-pool --server my-server --name my-deployment > deployment.yaml

  uv run create-prefect-deployment.py flows/my_flow.py:my_flow \\
    --work-pool my-pool --server my-server --name my-deployment | kubectl apply -f -

  # Full configuration
  uv run create-prefect-deployment.py flows/my_flow.py:my_flow \\
    --work-pool kubernetes-pool --server my-prefect-server \\
    --name hello-deployment --namespace production \\
    --description "Production deployment" --tag production --tag api \\
    --parameters '{"name": "Kubernetes"}' --concurrency-limit 5

Features:
- Automatic parameter schema generation from Python type hints
- No temporary files (unlike 'prefect deploy')
- Kubernetes native CRD output
    """
)




def generate_prefect_deployment_crd(
    flow_obj: Flow,
    name: str,
    work_pool: str,
    server: str,
    entrypoint: str,
    namespace: str = "default",
    description: Optional[str] = None,
    tags: Optional[List[str]] = None,
    parameters: Optional[Dict[str, Any]] = None,
    job_variables: Optional[Dict[str, Any]] = None,
    paused: bool = False,
    concurrency_limit: Optional[int] = None,
) -> Dict[str, Any]:
    """Generate a PrefectDeployment CRD manifest"""

    # Get parameter schema from the entrypoint
    parameter_schema_obj = parameter_schema_from_entrypoint(entrypoint)
    parameter_schema = parameter_schema_obj.model_dump() if parameter_schema_obj else None

    # Build the CRD
    crd = {
        "apiVersion": "prefect.io/v1",
        "kind": "PrefectDeployment",
        "metadata": {
            "name": name,
            "namespace": namespace,
        },
        "spec": {
            "server": {
                "name": server
            },
            "workPool": {
                "name": work_pool
            },
            "deployment": {
                "entrypoint": entrypoint
            }
        }
    }

    # Add optional deployment configuration
    deployment_config = crd["spec"]["deployment"]

    if description:
        deployment_config["description"] = description

    if tags:
        deployment_config["tags"] = tags

    if parameters:
        deployment_config["parameters"] = parameters

    if job_variables:
        deployment_config["jobVariables"] = job_variables

    if paused:
        deployment_config["paused"] = paused

    if concurrency_limit:
        deployment_config["concurrencyLimit"] = concurrency_limit

    if parameter_schema:
        deployment_config["parameterOpenApiSchema"] = parameter_schema

    return crd


@app.command()
def main(
    entrypoint: str = typer.Argument(
        ...,
        help="Flow entrypoint in format 'path/to/file.py:function_name'"
    ),
    work_pool: str = typer.Option(
        ...,
        "--work-pool",
        help="Name of the work pool to use for this deployment"
    ),
    server: str = typer.Option(
        ...,
        "--server",
        help="Name of the PrefectServer CRD to connect to"
    ),
    name: str = typer.Option(
        ...,
        "--name",
        help="Name for the deployment"
    ),
    namespace: str = typer.Option(
        "default",
        "--namespace",
        help="Kubernetes namespace for the deployment"
    ),
    description: Optional[str] = typer.Option(
        None,
        "--description",
        help="Description for the deployment"
    ),
    tags: Optional[List[str]] = typer.Option(
        None,
        "--tag",
        help="Tags for the deployment (can be specified multiple times)"
    ),
    parameters: Optional[str] = typer.Option(
        None,
        "--parameters",
        help="Default parameters as JSON string"
    ),
    job_variables: Optional[str] = typer.Option(
        None,
        "--job-variables",
        help="Job variables as JSON string"
    ),
    paused: bool = typer.Option(
        False,
        "--paused",
        help="Create deployment in paused state"
    ),
    concurrency_limit: Optional[int] = typer.Option(
        None,
        "--concurrency-limit",
        help="Maximum concurrent flow runs"
    ),
    output: Optional[Path] = typer.Option(
        None,
        "--output", "-o",
        help="Output file (defaults to stdout)"
    ),
    dry_run: bool = typer.Option(
        False,
        "--dry-run",
        help="Show what would be generated without creating files"
    ),
) -> None:
    """
    Convert a Prefect flow to a Kubernetes PrefectDeployment CRD manifest.

    This script introspects a Python flow file and generates a PrefectDeployment
    CRD that can be applied to Kubernetes to deploy the flow using the
    Prefect Operator.
    """

    try:
        # Load the flow using Prefect's built-in function
        typer.echo(f"Loading flow from: {entrypoint}", err=True)
        flow_obj = load_flow_from_entrypoint(entrypoint)
        typer.echo(f"Found flow: {flow_obj.name}", err=True)

        # Parse optional JSON parameters
        parsed_parameters = None
        if parameters:
            try:
                parsed_parameters = json.loads(parameters)
            except json.JSONDecodeError as e:
                raise typer.BadParameter(f"Invalid JSON for parameters: {e}")

        parsed_job_variables = None
        if job_variables:
            try:
                parsed_job_variables = json.loads(job_variables)
            except json.JSONDecodeError as e:
                raise typer.BadParameter(f"Invalid JSON for job-variables: {e}")

        # Generate the CRD
        crd = generate_prefect_deployment_crd(
            flow_obj=flow_obj,
            name=name,
            work_pool=work_pool,
            server=server,
            entrypoint=entrypoint,
            namespace=namespace,
            description=description,
            tags=tags,
            parameters=parsed_parameters,
            job_variables=parsed_job_variables,
            paused=paused,
            concurrency_limit=concurrency_limit,
        )

        # Convert to YAML
        yaml_output = yaml.dump(crd, default_flow_style=False, sort_keys=False)

        if dry_run:
            typer.echo("Dry run - would generate:", err=True)
            typer.echo(yaml_output)
            return

        # Output the result
        if output:
            output.write_text(yaml_output)
            typer.echo(f"Generated PrefectDeployment CRD: {output}", err=True)
        else:
            typer.echo(yaml_output)

    except Exception as e:
        typer.echo(f"Error: {e}", err=True)
        raise typer.Exit(1)


if __name__ == "__main__":
    app()
