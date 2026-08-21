module Runtime = Sqlc_ocaml_runtime_lwt

let database_url =
  Sys.getenv_opt "DATABASE_URL"
  |> Option.value ~default:"postgresql://override:override@localhost:5440/override_demo"

let fail error =
  prerr_endline (Caqti_error.show error);
  exit 1

let database_pool =
  lazy (Runtime.Pool.connect_uri_exn database_url)

let with_database operation =
  let pool = Lazy.force database_pool in
  match Runtime.Pool.run operation pool with
  | Ok value -> value
  | Error error -> fail error

let email value =
  match Email.of_string value with
  | Ok email -> email
  | Error message -> failwith message

let create display_name email =
  with_database (fun db ->
      Queries.CreateUser.execute db { display_name; email })
  |> ignore

let () =
  create "Ada" (Some (email "ada@example.com"));
  create "Grace" None;
  with_database (fun db -> Queries.ListUsers.execute db ())
  |> List.iter (fun (user : Queries.ListUsers.row) ->
         Printf.printf "%Ld %s %s\n" user.id user.display_name
           (match user.email with
           | Some address -> Email.to_string address
           | None -> "<none>"))
