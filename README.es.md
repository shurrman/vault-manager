# vault-manager

[English](README.md) · [Русский](README.ru.md) · [Español](README.es.md) ·
[简体中文](README.zh-CN.md)

![Vista previa de la interfaz de vault-manager](docs/images/vault-manager.svg)

`vault-manager` es un gestor de terminal interactivo e independiente para un
clúster de HashiCorp Vault. Se comunica únicamente con la API HTTP de Vault:
no requiere fuentes de identidad externas, sistemas de aprovisionamiento ni
sincronización específica de una instalación.

Versión actual: `0.2`.

Los archivos precompilados y sus sumas de comprobación están disponibles en la
[página de GitHub Releases](https://github.com/shurrman/vault-manager/releases/latest).

La interfaz utiliza tres paneles:

1. secciones como Groups, Users y Engines;
2. elementos de la sección seleccionada;
3. propiedades y acciones del elemento seleccionado.

Los nombres legibles para personas son los identificadores principales. Los
ID y accessors de Vault siguen disponibles en la vista previa cuando resultan
útiles para el diagnóstico.

## Funcionalidad actual

La versión 0.2 gestiona Status, Groups, Users, Policies, Engines, SSH,
Auth Methods, Audit y Tokens:

| Área | Acciones disponibles |
| --- | --- |
| Clúster | Seal, envío de claves unseal y restauración de snapshots Raft |
| Grupos internos | Crear, editar políticas/pertenencia/metadatos y eliminar |
| Usuarios | Crear userpass, editar entity y contraseña, revocar userpass y eliminar entity |
| Políticas ACL | Crear, editar contenido HCL/JSON multilínea y eliminar |
| Secrets engines | Leer/listar una ruta, habilitar y deshabilitar |
| Roles SSH | Crear, editar la configuración JSON completa y eliminar |
| Métodos de autenticación | Ajustar, habilitar y deshabilitar |
| Dispositivos de auditoría | Habilitar y deshabilitar |
| Tokens | Revocar por accessor |

Los valores multilínea conservan saltos e indentación en la vista previa. Cada
editor incluye una zona Help inferior con sintaxis, ejemplos y consejos. Las
acciones destructivas muestran una confirmación con Cancel seleccionado de
forma predeterminada; Vault sigue aplicando las capacidades ACL del token.

## Compilación

Para el desarrollo se necesita Go 1.25 o posterior. La compilación
predeterminada produce un binario Linux amd64 estático para el conjunto básico
de instrucciones x86-64:

```sh
make build
```

El resultado se encuentra en `bin/vault-manager`. La compilación utiliza
`CGO_ENABLED=0` y `GOAMD64=v1` para funcionar desde CentOS 7 hasta
Debian 13.

La matriz completa de archivos de la versión se genera con:

```sh
make release
```

| SO | Arquitectura | Archivo |
| --- | --- | --- |
| Linux | amd64 | `vault-manager_0.2_linux_amd64.tar.gz` |
| Linux | arm64 | `vault-manager_0.2_linux_arm64.tar.gz` |
| macOS | amd64 | `vault-manager_0.2_darwin_amd64.tar.gz` |
| macOS | arm64 | `vault-manager_0.2_darwin_arm64.tar.gz` |
| Windows | amd64 | `vault-manager_0.2_windows_amd64.zip` |

Todos los binarios utilizan `CGO_ENABLED=0`; las compilaciones amd64 usan
`GOAMD64=v1` y las arm64 usan el nivel básico `GOARM64=v8.0`.
`dist/checksums.txt` contiene las sumas SHA-256 de los cinco archivos. Al
publicar una etiqueta de dos partes `vX.Y`, GitHub Actions ejecuta las pruebas,
compila esta matriz y crea o actualiza automáticamente la versión en GitHub.

## Configuración

Copie `.vault-manager.example` como `.vault-manager`, sustituya la dirección
y el token, y limite el acceso al propietario del archivo:

```sh
cp .vault-manager.example .vault-manager
chmod 600 .vault-manager
```

```toml
[vault]
address = "https://vault.example.com:8200"
token = "replace-me"
request_timeout = "10s"

[ui]
color = "auto"
```

La configuración se rechaza si el grupo u otros usuarios pueden leerla. Tanto
`.vault-manager` como `.env` están excluidos de Git.

Una instalación que sincronice la pertenencia a grupos desde otro sistema
puede añadir un aviso local sin modificar la aplicación:

```toml
[deployment]
membership_notice = "Membership is managed externally and may change after synchronization."
```

Este texto se muestra antes de editar los grupos de un usuario. Es únicamente
informativo: el núcleo no llama ni interpreta el sistema externo.

## Ejecución

```sh
bin/vault-manager
```

La versión se muestra con `bin/vault-manager --version`. Para utilizar otro
archivo de configuración, indique `--config /path/to/config.toml`.

La comprobación de conectividad de solo lectura es:

```sh
bin/vault-manager --check
```

La comprobación enumera todas las secciones y muestra una vista previa de su
primer elemento. No realiza operaciones de escritura.

## Controles de teclado

- `↑`/`↓`, `Home`, `End`: desplazarse por el panel activo;
- `←`/`→`: desplazarse entre paneles;
- `Tab`/`Shift+Tab`: ir al panel de la derecha o de la izquierda; `Tab`
  desde el panel central selecciona la primera acción de la vista previa;
- `Enter`: activar un elemento, una acción o un campo del editor;
- `Space`: cambiar un campo booleano del editor;
- `Ctrl+S`: revisar la acción actual del editor;
- `Enter` en un campo multilínea: insertar una línea nueva;
- `Esc`: aceptar un campo multilínea o revisar/salir del editor;
- `r`: actualizar los datos activos;
- `?`: mostrar un recordatorio breve de las teclas;
- `q` o `Ctrl+C`: salir cuando no se está editando un campo de texto.

Se utiliza color cuando el terminal lo admite. Con `ui.color = "never"`, la
variable `NO_COLOR` o un terminal sin color, `*` marca la ruta seleccionada
y `>` el elemento activo. El tamaño mínimo del terminal es 80x20.

## Pertenencia a grupos y seguridad al guardar

Vault almacena la pertenencia directa de una entity en los objetos de grupos
internos. Por eso, cambiar el campo Groups de un usuario puede requerir varias
escrituras secuenciales y no es una operación atómica. Antes de escribir,
`vault-manager` comprueba si la entity o cualquiera de los grupos afectados
ha cambiado. Después actualiza los grupos en un orden determinista por nombre
y verifica cada resultado.

Si una escritura posterior falla, las escrituras anteriores que tuvieron éxito
no se revierten. La interfaz informa de las operaciones aplicadas, fallidas y
no intentadas, vuelve a cargar el estado real de Vault y exige abrir una nueva
edición antes de continuar.

Los demás editores también comparan el objeto actual con la versión cargada
originalmente y se niegan a sobrescribir cambios concurrentes. Las contraseñas
y los tokens nunca se muestran completos, se registran ni se incluyen en
mensajes de error.

## Comprobaciones de desarrollo

```sh
make test
make test-race
make vet
```

Las pruebas de escritura de la API utilizan un servidor HTTP de Vault simulado
dentro del proceso. El clúster configurado solo se utiliza mediante la
comprobación explícita de solo lectura `--check`.

Consulte [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) para conocer los detalles
de diseño (en inglés).
