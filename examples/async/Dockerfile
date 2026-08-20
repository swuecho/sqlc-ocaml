FROM ocaml/opam:debian-12-ocaml-5.2 AS build

RUN sudo apt-get update \
    && sudo apt-get install -y --no-install-recommends libpq-dev pkg-config \
    && sudo rm -rf /var/lib/apt/lists/*
RUN opam install -y dune caqti-async caqti-driver-postgresql async_unix uri

WORKDIR /home/opam/app
COPY --chown=opam:opam dune-project dune main.ml queries.ml queries.mli ./
RUN opam exec -- dune build ./main.exe

ENTRYPOINT ["opam", "exec", "--", "dune", "exec", "./main.exe", "--"]
