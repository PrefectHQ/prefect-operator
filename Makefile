.DEFAULT_GOAL := install

.bookkeeping/uv:
	mkdir -p .bookkeeping
	touch .bookkeeping/uv.next

	pip install -U pip uv

ifdef PYENV_VIRTUAL_ENV
	pyenv rehash
endif

	mv .bookkeeping/uv.next .bookkeeping/uv

.bookkeeping/development.txt: .bookkeeping/uv requirements-dev.txt pyproject.toml
	mkdir -p .bookkeeping
	cat requirements-dev.txt > .bookkeeping/development.txt.next

	uv pip sync .bookkeeping/development.txt.next
	uv pip install -e .

ifdef PYENV_VIRTUAL_ENV
	pyenv rehash
endif

	mv .bookkeeping/development.txt.next .bookkeeping/development.txt

requirements.txt: requirements.in .bookkeeping/uv
	uv pip compile requirements.in --output-file $@

requirements-dev.txt: requirements.txt requirements-dev.in .bookkeeping/uv
	uv pip compile requirements.txt requirements-dev.in --output-file $@

.git/hooks/pre-commit: .bookkeeping/development.txt
	pre-commit install

.pre-commit-config.yaml: .bookkeeping/development.txt
	./sync-pre-commit

.PHONY: docker
docker: Dockerfile .dockerignore requirements.txt
	docker build -t prefect-operator:latest .

.PHONY: install
install: .bookkeeping/development.txt .git/hooks/pre-commit .pre-commit-config.yaml docker

.PHONY: clean
clean:
	rm -Rf .bookkeeping/
