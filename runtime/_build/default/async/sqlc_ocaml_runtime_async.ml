(** Helpers for creating and using a reusable Caqti Async connection pool. *)
module Pool = struct
  include Caqti_async.Pool

  let connect = Caqti_async.connect_pool

  let connect_uri database_url = connect (Uri.of_string database_url)

  let connect_uri_exn database_url =
    match connect_uri database_url with
    | Ok pool -> pool
    | Error error -> failwith (Caqti_error.show error)

end

(** Resource-first database operations for application code. *)
module Database = struct
  let connect = Pool.connect
  let connect_uri = Pool.connect_uri
  let connect_uri_exn = Pool.connect_uri_exn

  let use ?priority database operation =
    Pool.use ?priority operation database
end

let with_database ?priority database operation =
  Database.use ?priority database operation
