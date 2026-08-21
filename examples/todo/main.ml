module Runtime = Sqlc_ocaml_runtime_lwt

let database_url =
  Sys.getenv_opt "DATABASE_URL"
  |> Option.value ~default:"postgresql://todo:todo@localhost:5438/todo"

let fail error =
  prerr_endline (Caqti_error.show error);
  exit 1

let database_pool =
  lazy (Runtime.Pool.connect_uri_exn database_url)

let run_database operation =
  let pool = Lazy.force database_pool in
  Runtime.Pool.run operation pool

let with_database operation =
  match run_database operation with
  | Ok value -> value
  | Error error -> fail error

let with_database_execrows operation =
  match run_database operation with
  | Ok value -> value
  | Error `Unsupported ->
      prerr_endline "database driver does not report affected row counts";
      exit 1
  | Error (#Caqti_error.t as error) -> fail error

let print_todo (todo : Queries.ListTodos.row) =
  Printf.printf "%Ld [%s] %s tags=%s\n" todo.id
    (if todo.completed then "x" else " ") todo.title
    (String.concat "|" todo.tags)

let usage () =
  prerr_endline "Usage: todo add TITLE | list | ids ID... | done ID | delete ID";
  exit 2

let parse_id value =
  match Int64.of_string_opt value with Some id -> id | None -> usage ()

let add title =
  let todo =
    with_database (fun db ->
        Queries.CreateTodo.execute db
          { title; tags = [ "cli"; "comma,value"; "quote\"slash\\" ] })
  in
  Printf.printf "Created todo %Ld: %s\n" todo.id todo.title

let list () =
  let todos =
    with_database (fun db -> Queries.ListTodos.execute db ())
  in
  List.iter print_todo todos

let list_ids values =
  let ids = List.map parse_id values in
  let todos =
    with_database (fun db -> Queries.ListTodosByIds.execute db { ids })
  in
  List.iter
    (fun (todo : Queries.ListTodosByIds.row) ->
      Printf.printf "%Ld [%s] %s\n" todo.id
        (if todo.completed then "x" else " ") todo.title)
    todos

let done_ id =
  let todo =
    with_database (fun db -> Queries.CompleteTodo.execute db { id })
  in
  Printf.printf "Completed todo %Ld: %s\n" todo.id todo.title

let delete id =
  let affected =
    with_database_execrows (fun db -> Queries.DeleteTodo.execute db { id })
  in
  Printf.printf "Deleted %d todo(s)\n" affected

let () =
  match Array.to_list Sys.argv with
  | [ _; "add"; title ] when String.trim title <> "" -> add title
  | [ _; "list" ] -> list ()
  | _ :: "ids" :: ids -> list_ids ids
  | [ _; "done"; id ] -> done_ (parse_id id)
  | [ _; "delete"; id ] -> delete (parse_id id)
  | _ -> usage ()
