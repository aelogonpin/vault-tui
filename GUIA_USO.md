# Guía de uso — vault-tui

TUI interactiva para explorar y gestionar secretos en motores **KV v2** de
HashiCorp Vault: navegar carpetas, ver un secreto y su historial de
versiones, crear secretos, guardar ediciones como nueva versión, renombrar
secretos/carpetas y borrar (soft-delete o destrucción permanente).

---

## 1. Requisitos

- Go 1.26+ (`brew install go` si no lo tienes)
- Acceso de red a tu servidor Vault
- El método de auth **OIDC** habilitado en ese Vault (por ahora es el único
  método de login soportado)

## 2. Compilar

```sh
cd ~/vault-tui
go build -o vault-tui .
```

Esto genera el binario `vault-tui` en la carpeta del proyecto.

## 3. Arrancar

```sh
./vault-tui --addr https://vault.example.com
```

También puedes fijar variables de entorno en vez de flags:

| Flag           | Variable de entorno | Por defecto | Notas |
|----------------|----------------------|-------------|-------|
| `--addr`       | `VAULT_ADDR`         | —           | obligatorio |
| `--namespace`  | `VAULT_NAMESPACE`    | —           | solo Vault Enterprise |
| `--oidc-mount` | `VAULT_OIDC_MOUNT`   | `oidc`      | mount del método OIDC — confírmalo con el admin de tu Vault si no es `oidc` |
| `--oidc-role`  | `VAULT_OIDC_ROLE`    | —           | vacío = usa el rol por defecto del mount |
| `--oidc-port`  | `VAULT_OIDC_PORT`    | `8250`      | puerto local de callback; ver §5 |
| `--debug-log`  | `VAULT_TUI_DEBUG_LOG` | —          | vuelca logs de depuración a este fichero; ver §9.1 |

Ejemplo dejándolo fijado en tu shell:

```sh
export VAULT_ADDR=https://vault.example.com
./vault-tui
```

---

## 4. Qué pasa al arrancar (autenticación)

1. La app busca un token cacheado en `~/.vault-token` (el mismo fichero que
   usa el CLI oficial `vault`, así que ambos comparten sesión). Si es
   válido, entra directo sin pedir login.
2. Si no hay token o ha caducado, abre tu navegador contra el proveedor
   OIDC que tengas configurado en Vault. Completas el login ahí (o se
   completa solo si ya tenías sesión activa de SSO).
3. Al terminar verás en el navegador "**Login successful — you can close
   this tab**". Vuelves a la terminal y la TUI arranca sola.
4. El token nuevo se guarda en `~/.vault-token` para la próxima vez.

**Importante:** este paso abre un navegador real y puede completar un login
de verdad si ya tienes sesión SSO activa — no es una simulación.

### Forzar un nuevo login

Borra el token cacheado y vuelve a arrancar:

```sh
rm ~/.vault-token
./vault-tui --addr https://vault.example.com
```

---

## 5. Detalle del puerto de callback OIDC (8250)

La app monta un pequeño servidor HTTP local para recibir la redirección del
login (`http://localhost:8250/oidc/callback`), exactamente el mismo puerto
que usa `vault login -method=oidc`. Se hace así a propósito: es muy
probable que ese puerto ya esté en la lista `allowed_redirect_uris` del
role OIDC en Vault y en la app registrada en tu proveedor OIDC, así que
funciona sin tocar nada.

Si el puerto 8250 está ocupado por otro proceso, la app prueba
automáticamente 8251–8259. Si necesitas forzar uno concreto (por ejemplo
porque solo ese está permitido en tu configuración OIDC):

```sh
./vault-tui --addr https://vault.example.com --oidc-port 8260
```

En ese caso tendrías que añadir `http://localhost:8260/oidc/callback` a la
lista de redirect URIs permitidas en Vault y en tu proveedor OIDC — si no,
el login fallará con un error del proveedor sobre "redirect_uri no coincide".

---

## 6. Navegación general

```
Mounts (motores KV) → Explorador de carpetas/secretos → Detalle de secreto → Historial de versiones
```

- **Mounts**: lista los motores `kv` montados. Solo se puede entrar en los
  de versión 2 (versionados). Los de v1 aparecen listados pero no se
  pueden abrir.
- **Explorador**: dentro de un mount, navegas por carpetas (que en Vault
  KV v2 no son más que prefijos de path) y secretos.
- **Detalle de secreto**: muestra las claves/valores de la versión
  seleccionada (por defecto la última).
- **Historial de versiones**: lista todas las versiones con su fecha y si
  están borradas/destruidas; puedes abrir cualquiera en modo lectura.

## 7. Atajos de teclado

| Pantalla | Teclas |
|---|---|
| Mounts | `↑/↓` moverse · `enter` abrir · `/` filtrar · `q` salir |
| Explorador | `enter` abrir · `n` nuevo secreto · `N` nueva carpeta · `e` editar (nueva versión, solo secretos) · `c` copiar · `x` mover (secretos o carpetas enteras) · `r` renombrar · `d`/`D` borrar/destruir (solo secretos) · `esc`/`backspace` subir un nivel |
| Explorador (con copia/movimiento armado) | `p` pegar en la carpeta actual · `esc` cancelar — la navegación (`enter`, subir nivel, cambiar de mount) funciona con normalidad para poder llegar a cualquier destino |
| Detalle de secreto | `m` mostrar/ocultar valores · `e` editar · `c` copiar · `x` mover · `v` historial de versiones · `r` renombrar · `d` borrar · `D` destruir · `esc` volver |
| Historial de versiones | `↑/↓` moverse · `enter` ver esa versión (solo lectura) · `esc` volver |
| Editor de claves/valores | escribir para editar · `tab`/`shift+tab` cambiar de campo · `↑/↓` cambiar de fila · `ctrl+n` añadir fila · `ctrl+x` quitar fila · `ctrl+s` guardar · `esc` cancelar |
| Prompts de nombre/rename | escribir · `enter` confirmar · `esc` cancelar |
| Confirmación | `y` confirmar · `n`/`esc` cancelar |

La línea de ayuda de abajo del explorador es **contextual**: solo muestra
los atajos que de verdad aplican al elemento seleccionado. Si tienes una
carpeta seleccionada, `e`/`d`/`D` desaparecen (no se pueden editar/borrar
directamente, solo renombrar); en cuanto seleccionas un secreto, vuelven a
aparecer.

**`ctrl+c` cierra la app desde cualquier pantalla, sin excepción** — incluso
con un campo de texto enfocado (editor, rename, prompt de nuevo nombre). Es
el único atajo garantizado en todos los contextos. `q` también cierra, pero
solo en las pantallas de navegación (mounts, explorador, detalle de
secreto) y nunca mientras estás escribiendo, para poder usar la letra "q"
con normalidad en nombres/valores/filtros.

### Filtro (`/`)

El filtro usa coincidencia por **subcadena** (no *fuzzy*): al escribir,
solo quedan los elementos cuyo nombre contiene literalmente lo que has
tecleado, en orden y sin huecos — así "sec" solo casa con nombres que
contienen "sec", no con cualquier cosa que tenga esas letras sueltas por
medio. Mientras el filtro está activo, todas las teclas (incluida `q`, `n`,
`e`, `r`, `d`, `h`...) van al cuadro de búsqueda, no a los atajos de la
pantalla.

---

## 8. Operaciones explicadas

### Crear un secreto nuevo
En el explorador, `n` → escribe el path relativo a la carpeta actual
(puede incluir `/` para crear subcarpetas implícitas, ya que en Vault KV v2
las carpetas no existen como objeto, solo como prefijo de path) → `enter`
→ te lleva al editor de claves/valores → añade pares con `ctrl+n` →
`ctrl+s` para guardar.

### Crear una carpeta vacía
En el explorador, `N` (mayúscula) → escribe el nombre (puede incluir `/`
para anidar, p. ej. `equipo/proyecto`) → `enter`. A diferencia de `n`, esto
no te lleva al editor: la carpeta se crea al momento y vuelves al
explorador viéndola ya en la lista.

Por debajo, como Vault KV v2 no tiene un objeto "carpeta" real (el `LIST`
de Vault solo devuelve prefijos que tienen al menos un secreto real
dentro), lo que hace `N` es crear un secreto placeholder llamado `.keep`
dentro de la carpeta nueva — es lo único que hace que la carpeta aparezca
en los listados. Lo verás en el explorador marcado como
"🧷 .keep (placeholder — safe to delete)": es un secreto normal, así que
puedes borrarlo tú mismo con `d`/`D` en cuanto añadas algo real ahí dentro,
o dejarlo — no molesta.

### Editar / crear nueva versión
Cada vez que guardas con `ctrl+s`, Vault crea una **versión nueva completa**
del secreto (no hay parcheo campo a campo a nivel de almacenamiento). Por
eso el editor precarga todas las claves existentes: si borras una fila y
guardas, esa clave desaparece en la nueva versión.

El editor muestra dos columnas (KEY / VALUE) con la fila activa resaltada
en azul y el resto atenuadas, para que se vea claro en qué campo estás
escribiendo — clave o valor.

#### Valores: texto plano o JSON
El campo de valor acepta lo que ya tecleabas (texto plano — contraseñas,
tokens, URLs...) y además **JSON real**: números, `true`/`false`, `null`,
objetos `{"a":1}` o arrays `["x","y"]`. A la derecha de cada fila aparece
una etiqueta en vivo que te dice cómo se va a guardar ese valor:

- `text` — se guarda tal cual, como string (comportamiento de siempre).
- `json·number`, `json·bool`, `json·null`, `json·object`, `json·array`,
  `json·string` — se guarda como ese tipo JSON.

No hace falta activar nada: si lo que escribes es JSON válido, se
interpreta como JSON; si no, se guarda como texto plano — así que valores
normales como `admin` o `hunter2!` siguen funcionando exactamente igual que
antes, sin sorpresas.

**Truco para forzar texto:** si tu secreto es un valor que *parece* JSON
pero en realidad debe ser texto (por ejemplo un PIN `12345` o un código
`true`), enciérralo entre comillas: `"12345"`. Así se guarda como string
`"12345"` en vez de como el número `12345`. Y al revés: cuando reabras un
secreto así, la app ya lo muestra con las comillas puestas automáticamente,
precisamente para que si lo guardas sin tocarlo no cambie de tipo por
accidente.

### Ver historial de versiones
Desde el detalle de un secreto, `v`. Selecciona una versión y `enter` para
verla en modo lectura (no la convierte en la versión activa — para eso
tendrías que copiar sus valores al editor manualmente y guardar).

### Mostrar/ocultar valores
En el detalle de un secreto, los valores aparecen tapados por defecto
(`••••••••`) para que no se vean a simple vista (por encima del hombro,
capturas de pantalla, etc.). Pulsa `m` para revelarlos y `m` otra vez para
volver a ocultarlos. Cada vez que abres un secreto o cambias de versión,
vuelve a arrancar oculto — es una medida de seguridad por defecto, no algo
que se quede "desbloqueado" sin querer. El editor (`e`) sí muestra el valor
real siempre, porque lo necesitas para poder editarlo.

### Renombrar un secreto o una carpeta
`r` sobre el elemento seleccionado en el explorador (o desde el detalle de
un secreto), escribe el nuevo nombre, `enter`. **Funciona igual para
carpetas que para secretos** — si seleccionas una carpeta, mueve *todos*
los secretos que tenga dentro, uno por uno, al nuevo nombre (Vault no
permite renombrar una carpeta sin tocar cada objeto de dentro, así que la
app ya lo hace automáticamente por ti).

**La ruta padre está bloqueada a propósito.** El prompt te muestra la
carpeta contenedora en gris, fija, y solo puedes escribir el nombre final
(ej. si estás renombrando `prueba/secreto`, ves `prueba/` fijo y editas
solo `secreto`). No se puede editar ni borrar esa parte por accidente. Esto
existe porque antes se podía editar el path completo — y bastaba con
borrar de más para mover sin querer un secreto (o una carpeta entera) fuera
de su sitio y liarla. Si necesitas mover un secreto o una carpeta entera a
un sitio *libremente elegido* (no solo renombrarlo en el mismo lugar) —
incluida otra carpeta, otro mount o otro cliente — usa copiar/mover (`c`/`x`
en el explorador), que sigue a continuación.

### Copiar o mover un secreto — o una carpeta entera — a otro sitio (incluso otro cliente/mount)
Esto es distinto de renombrar: aquí sí eliges libremente el destino,
incluso en un motor KV (mount) completamente distinto — por ejemplo, si tu
rol tiene acceso a los mounts de varios clientes, puedes coger un secreto
(o una carpeta entera con todo lo que tenga dentro) de uno y llevarlo a
otro sin salir de la app.

1. Selecciona un secreto **o una carpeta** en el explorador y pulsa `c`
   (copiar, mantiene el original) o `x` (mover, borra el original tras
   copiar con éxito). Sobre una carpeta, esto arrastra recursivamente todos
   los secretos que tenga dentro, en cualquier nivel de subcarpetas.
2. La app te deja en el explorador con un aviso arriba ("Copying/Moving
   ... — browse to a destination..."). Navega con normalidad: `enter` para
   entrar en carpetas, `esc`/`backspace` para subir, incluso puedes volver
   a la lista de mounts y meterte en uno completamente distinto.
3. Cuando estés en la carpeta destino, pulsa `p` para pegar ahí (se guarda
   con el mismo nombre que tenía). `esc` en cualquier momento cancela la
   operación entera sin tocar nada.

Si ya existe algo con ese nombre en el destino, te pide confirmación antes
de sobrescribir (crea una versión nueva ahí, no borra las anteriores). Si
intentas pegar justo en el mismo sitio de origen con `x`, la app lo
rechaza — moverlo sobre sí mismo lo borraría sin dejar copia. Con carpetas
aplica también dentro de sus propias subcarpetas: no puedes pegar una
carpeta dentro de sí misma ni de ninguna carpeta que cuelgue de ella, por
la misma razón (el borrado del origen al final del "mover" se llevaría por
delante la copia recién pegada).

Con carpetas, si algo falla a mitad de camino (por ejemplo, se corta la
conexión), el mensaje de error te dice cuántos secretos se llegaron a
copiar/mover, para que puedas terminar el resto a mano o deshacerlo.

⚠️ **Vault KV v2 no tiene un "rename" atómico nativo.** Esta operación en
realidad hace: leer el dato actual → escribirlo en el path nuevo (empieza
un historial de versiones nuevo ahí) → borrar toda la metadata/versiones
del path viejo. Consecuencias:

- **El historial de versiones antiguo se pierde.** La app avisa de esto
  antes de confirmar.
- Renombrar una **carpeta** mueve *todos* los secretos que haya debajo,
  uno a uno (copia todos primero, luego borra los viejos). Con muchos
  secretos puede tardar unos segundos.
- Si falla a mitad de camino, el mensaje de error te dice cuántos secretos
  se movieron ya, para que puedas terminar el resto a mano o revertir.

### Borrar (`d`) vs Destruir (`D`)
- **`d` — soft-delete**: borra la última versión, pero es recuperable
  (Vault la mantiene hasta que se destruye explícitamente o expira por
  política). Esta app no tiene UI de "undelete" todavía — habría que
  hacerlo con el CLI/API de Vault directamente.
- **`D` — destroy permanente**: borra el secreto y **todo** su historial de
  versiones sin posibilidad de recuperación. Pide confirmación con aviso
  en rojo.
- No hay borrado recursivo de carpetas por diseño (para evitar cargarte un
  subárbol entero sin querer).

---

## 9. Cosas a tener en cuenta / limitaciones

- **Solo motores KV v2.** Los de v1 se listan pero no se pueden abrir — no
  tienen versionado, que es gran parte de lo que aporta esta app.
- **Los valores se editan como texto plano.** Vault permite técnicamente
  cualquier escalar JSON como valor; el editor los aplana todos a string.
  Para la inmensa mayoría de secretos (contraseñas, tokens, connection
  strings) no afecta; si un secreto tuviera un valor no-string (número,
  booleano) y lo vuelves a guardar desde aquí, saldrá como string.
- **El renombrado no es atómico** (ver arriba) y **no preserva el
  historial de versiones** del path original.
- **Sin undo/rollback en la UI** para versiones borradas — se puede hacer
  con el CLI de Vault mientras tanto.
- **El token se comparte con el CLI de `vault`** vía `~/.vault-token`. Si
  quieres cerrar sesión del todo: `rm ~/.vault-token` (y opcionalmente
  revocar el token con `vault token revoke -self`).

### 9.1 Logs de depuración

Como la TUI ocupa toda la terminal, no se puede simplemente imprimir por
consola para depurar. Arranca con `--debug-log` (o `VAULT_TUI_DEBUG_LOG`)
apuntando a un fichero, y en otra terminal haz `tail -f` sobre él mientras
usas la app:

```sh
./vault-tui --addr https://vault.example.com --debug-log /tmp/vault-tui.log
# en otra terminal:
tail -f /tmp/vault-tui.log
```

Ahí verás cada tecla pulsada, el resultado de cada filtro (cuántos items
coincidieron) y el detalle de cualquier error de operación contra Vault
(carga de mounts/entradas/secretos, guardado, borrado, renombrado). Sin
`--debug-log`, no se escribe nada (para no ensuciar la pantalla de la TUI).

---

## 10. Problemas frecuentes

| Síntoma | Causa probable | Solución |
|---|---|---|
| Error de conexión / "no Vault address configured" | Falta `--addr` o `VAULT_ADDR` | Pásalo explícitamente |
| El navegador da error de `redirect_uri` no permitida | El puerto usado no está whitelisted en el role OIDC / en tu proveedor OIDC | Usa `--oidc-port` con uno permitido, o pide al admin de Vault que añada `http://localhost:8250/oidc/callback` |
| Se queda esperando el login sin abrir navegador | El comando `open` no encontró un navegador por defecto, o estás en una sesión sin entorno gráfico | Copia manualmente la URL que imprime la terminal y ábrela tú |
| "OIDC login failed: ... role required" o similar | El mount OIDC no tiene rol por defecto | Especifica `--oidc-role <nombre>` |
| Un mount KV v2 no aparece en la lista | El token no tiene permiso de `list` sobre `sys/mounts` o sobre ese path | Revisa la policy asociada a tu usuario/rol en Vault |
| "could not open a new TTY" | Intentaste correr el binario sin una terminal real (p. ej. en background/redirigido) | Ejecútalo directamente en tu terminal, no en background |

---

## 11. Estructura del proyecto (referencia rápida)

```
main.go                    flags/env, bootstrap de auth, arranque de la TUI
internal/vault/
  config.go                 struct de configuración
  client.go                 cliente Vault + caché de token
  oidc.go                   flujo de login OIDC vía navegador
  kv.go                     operaciones KV v2 (listar/leer/escribir/borrar/renombrar)
internal/tui/
  model.go                  modelo raíz, máquina de estados de pantallas
  mounts.go / browse.go / secret.go   pantallas de navegación
  editor.go / newname.go / rename.go / confirm.go   pantallas de escritura
  items.go, messages.go, styles.go    soporte (list.Item, comandos async, estilos)
```
