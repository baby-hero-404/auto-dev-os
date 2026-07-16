; Adapted from references/aider/aider/queries (aider/repomap.py's own tag source, this project's
; stated inspiration — see docs/features/engineering/01-context-management.md), extended with
; constructor_declaration (Aider's query omits constructors; a constructor is a definition worth
; surfacing too).

(class_declaration name: (identifier) @name.definition.class) @definition.class
(interface_declaration name: (identifier) @name.definition.interface) @definition.interface
(method_declaration name: (identifier) @name.definition.method) @definition.method
(constructor_declaration name: (identifier) @name.definition.method) @definition.method
(method_invocation name: (identifier) @name.reference.call)
(object_creation_expression type: (type_identifier) @name.reference.call)
(superclass (type_identifier) @name.reference.call)
(type_list (type_identifier) @name.reference.call)
