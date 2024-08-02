FROM python:3.12.4-slim-bookworm

RUN apt-get update && apt-get install -y git && apt-get clean

WORKDIR /app
ENTRYPOINT [ "prefect-operator" ]
CMD [ "run" ]

RUN pip install -U pip uv

COPY requirements.txt .

RUN uv pip install --system -r requirements.txt

COPY pyproject.toml .
COPY src src

RUN --mount=source=.git,target=.git,type=bind pip install --no-cache-dir -e .

RUN prefect-operator --version
