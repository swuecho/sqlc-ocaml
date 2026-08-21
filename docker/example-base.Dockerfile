FROM ocaml/opam:debian-12-ocaml-5.2

RUN sudo apt-get update \
    && sudo apt-get install -y --no-install-recommends \
      libev-dev \
      libpq-dev \
      pkg-config \
    && sudo rm -rf /var/lib/apt/lists/*

RUN opam install -y \
      async_unix \
      caqti-async \
      caqti-driver-postgresql \
      caqti-lwt \
      dream \
      dune \
      lwt \
      ptime \
      uri \
      uuidm \
      yojson

COPY --chown=opam:opam runtime /home/opam/sqlc-ocaml-runtime
RUN cd /home/opam/sqlc-ocaml-runtime \
    && opam exec -- dune build @install \
    && opam exec -- dune install
