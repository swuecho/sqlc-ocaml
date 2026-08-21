ARG EXAMPLE_BASE_IMAGE=sqlc-ocaml-example-base:local
FROM ${EXAMPLE_BASE_IMAGE}

WORKDIR /home/opam/app
COPY --chown=opam:opam dune-project dune main.ml queries.ml queries.mli sqlc_runtime.ml sqlc_runtime.mli ./
RUN opam exec -- dune build ./main.exe

ENTRYPOINT ["opam", "exec", "--", "dune", "exec", "./main.exe", "--"]
