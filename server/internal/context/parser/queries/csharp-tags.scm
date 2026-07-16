; Adapted from references/aider/aider/queries, verified against the actual grammar — node names
; matched directly (class_declaration/interface_declaration/method_declaration/
; namespace_declaration are already the full definition node, so no separate span capture is
; needed the way C/C++ function_declarator required one).

(class_declaration name: (identifier) @name.definition.class) @definition.class
(interface_declaration name: (identifier) @name.definition.interface) @definition.interface
(method_declaration name: (identifier) @name.definition.method) @definition.method
(namespace_declaration name: (identifier) @name.definition.module) @definition.module
(object_creation_expression type: (identifier) @name.reference.call)
(invocation_expression function: (identifier) @name.reference.call)
(invocation_expression function: (member_access_expression name: (identifier) @name.reference.call))
