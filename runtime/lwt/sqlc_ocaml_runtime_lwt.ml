(** Helpers for creating and using a reusable Caqti Lwt connection pool. *)
module Pool = struct
  include Caqti_lwt_unix.Pool

  let connect = Caqti_lwt_unix.connect_pool

  let connect_uri database_url = connect (Uri.of_string database_url)

  let connect_uri_exn database_url =
    match connect_uri database_url with
    | Ok pool -> pool
    | Error error -> failwith (Caqti_error.show error)

  let run ?priority operation pool =
    Lwt_main.run (use ?priority operation pool)
end
