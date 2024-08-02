import argparse
import importlib
import os
import sys

import yaml

from prefect_operator import __version__
from prefect_operator.resources import CustomResource

parser = argparse.ArgumentParser(description="Prefect Operator")

subparsers = parser.add_subparsers(dest="command", help="Available commands")

parser.add_argument("--version", action="version", version=__version__)
subparsers.add_parser("version", help="Print the version")

run_parser = subparsers.add_parser("run", help="Run prefect-operator")

generate_crds_parser = subparsers.add_parser(
    "generate-crds",
    help="Generate the Custom Resource Definitions",
)

args = parser.parse_args()

modules = [
    "prefect_operator.server",
    "prefect_operator.work_pool",
]


def main():
    match args.command:
        case "run":
            os.execvp(
                "kopf",
                [
                    "kopf",
                    "run",
                    "--all-namespaces",
                ]
                + [(f"--module={module}") for module in modules],
            )
        case "generate-crds":
            for module in modules:
                importlib.import_module(module)

            yaml.dump_all(CustomResource.definitions(), stream=sys.stdout)
        case "version":
            print(__version__)
        case _:
            parser.print_help()


if __name__ == "__main__":
    main()
