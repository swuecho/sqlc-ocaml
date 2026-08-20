FROM ocaml/opam:debian-12-ocaml-5.2 AS build

RUN sudo apt-get update \
    && sudo apt-get install -y --no-install-recommends libpq-dev pkg-config \
    && sudo rm -rf /var/lib/apt/lists/*
RUN opam install -y dune caqti-lwt caqti-driver-postgresql lwt uri

WORKDIR /home/opam/app
COPY --chown=opam:opam dune-project dune email.ml main.ml queries.ml queries.mli ./
RUN opam exec -- dune build ./main.exe

ENTRYPOINT ["opam", "exec", "--", "dune", "exec", "./main.exe", "--"]
