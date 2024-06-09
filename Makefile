.DEFAULT_GOAL := install

.bookkeeping/uv:
	mkdir -p .bookkeeping
	touch .bookkeeping/uv.next

	pip install -U pip uv

ifdef PYENV_VIRTUAL_ENV
	pyenv rehash
endif

	mv .bookkeeping/uv.next .bookkeeping/uv

.bookkeeping/development.txt: .bookkeeping/uv requirements.txt
	mkdir -p .bookkeeping
	cat requirements.txt > .bookkeeping/development.txt.next

	uv pip sync .bookkeeping/development.txt.next

ifdef PYENV_VIRTUAL_ENV
	pyenv rehash
endif

	mv .bookkeeping/development.txt.next .bookkeeping/development.txt

%.txt: %.in .bookkeeping/uv
	uv pip compile --resolver=backtracking --upgrade --output-file $@ $<

.git/hooks/pre-commit: .bookkeeping/development.txt
	pre-commit install

.pre-commit-config.yaml: .bookkeeping/development.txt
	./sync-pre-commit

.PHONY: install
install: .bookkeeping/development.txt .git/hooks/pre-commit .pre-commit-config.yaml

.PHONY: clean
clean:
	rm -Rf .bookkeeping/
