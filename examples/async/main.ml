open Async_kernel
open Async_unix

let database_url =
  Stdlib.Sys.getenv_opt "DATABASE_URL"
  |> Option.value ~default:"postgresql://async:async@localhost:5441/async_demo"

let fail error = failwith (Caqti_error.show error)

let verify db =
  Queries.ListNumbers.execute db ()
  >>= function
  | Error error -> fail error
  | Ok rows ->
      if List.length rows <> 1000 then
        failwith (Printf.sprintf "execute returned %d rows, expected 1000" (List.length rows));
      Queries.ListNumbers.fold db () ~init:(0, 0L)
        ~f:(fun (row : Queries.ListNumbers.row) (count, sum) ->
          (count + 1, Int64.add sum row.value))
      >>= function
      | Error error -> fail error
      | Ok (count, sum) ->
          if count <> 1000 || sum <> 500500L then
            failwith
              (Printf.sprintf "fold returned count=%d sum=%Ld" count sum);
          let stdout = Lazy.force Writer.stdout in
          Writer.writef stdout
            "Async execute and fold verified %d rows (sum=%Ld).\n" count sum;
          Writer.flushed stdout

let main () =
  Caqti_async.connect (Uri.of_string database_url)
  >>= function
  | Error error -> fail error
  | Ok db -> verify db

let scheduler_main () =
  don't_wait_for
    (main ()
    >>| fun () -> Shutdown.shutdown 0)

let () = Stdlib.ignore (Scheduler.go_main ~main:scheduler_main ())
