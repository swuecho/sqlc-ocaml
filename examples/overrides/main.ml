let database_url =
  Sys.getenv_opt "DATABASE_URL"
  |> Option.value ~default:"postgresql://override:override@localhost:5440/override_demo"

let fail error =
  prerr_endline (Caqti_error.show error);
  exit 1

let with_database operation =
  match Caqti_lwt_unix.connect_pool (Uri.of_string database_url) with
  | Error error -> fail error
  | Ok pool ->
      match Lwt_main.run (Caqti_lwt_unix.Pool.use operation pool) with
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
