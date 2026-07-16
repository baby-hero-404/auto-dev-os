; Adapted from references/aider/aider/queries, verified against the actual grammar.

(function_definition name: (word) @name.definition.function) @definition.function
(variable_assignment name: (variable_name) @name.definition.variable) @definition.variable
(command name: (command_name) @name.reference.call)
