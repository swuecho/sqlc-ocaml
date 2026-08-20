FROM ocaml/opam:debian-12-ocaml-5.2

RUN sudo apt-get update \
    && sudo apt-get install -y --no-install-recommends libev-dev libpq-dev pkg-config \
    && sudo rm -rf /var/lib/apt/lists/*
RUN opam install -y dune dream caqti-lwt caqti-driver-postgresql lwt yojson uri

WORKDIR /home/opam/app
COPY --chown=opam:opam dune-project dune main.ml queries.ml queries.mli ./
RUN opam exec -- dune build ./main.exe

ENTRYPOINT ["opam", "exec", "--", "dune", "exec", "./main.exe"]
