package main

import (
    "bufio"
    "bytes"
    "fmt"
    "log"
    "math/rand"
    "net"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"
)

type Plane struct {
    id       int
    numPass  int
    category string
    arrival  time.Time
}

type Runway struct {
    id int
}

type Gate struct {
    id int
}

type PriorityQueue []Plane

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
    // Define priority based on category
    categoryPriority := map[string]int{"A": 1, "B": 2, "C": 3}
    if pq[i].category != pq[j].category {
        return categoryPriority[pq[i].category] < categoryPriority[pq[j].category]
    }
    return pq[i].arrival.Before(pq[j].arrival)
}

func (pq PriorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }

type Airport struct {
    mu             sync.Mutex
    runways        chan Runway
    gates          chan Gate
    queue          PriorityQueue
    priority       string // Current priority category (e.g., "A", "B", "C")
    state          int    // Current state (0-9)
    maxQueueCap    int
    activeLandings int
    activeGates    int
    totalPlanes    int
    landedPlanes   int
    done           chan bool
}

var (
    buf     bytes.Buffer
    logger  = log.New(&buf, "logger: ", log.Lshortfile)
    wg      sync.WaitGroup
    runways = 3
    gates   = 10
)

func main() {
    rand.Seed(time.Now().UnixNano())

    // Configuración inicial de aviones
    numA := 20
    numB := 5
    numC := 5
    totalPlanes := numA + numB + numC

    conn, err := net.Dial("tcp", "localhost:8000")
    if err != nil {
        logger.Fatal(err)
    }
    defer conn.Close()

    airport := &Airport{
        runways:     make(chan Runway, runways),
        gates:       make(chan Gate, gates),
        queue:       PriorityQueue{},
        priority:    "",
        state:       0,
        maxQueueCap: 50,
        done:        make(chan bool),
        totalPlanes: totalPlanes,
    }

    // Inicializar pistas
    for i := 1; i <= runways; i++ {
        airport.runways <- Runway{i}
    }

    // Inicializar puertas
    for i := 1; i <= gates; i++ {
        airport.gates <- Gate{i}
    }

    // Generar aviones iniciales en orden aleatorio
    go generateInitialAirplanes(airport, numA, numB, numC)

    // Manejar mensajes entrantes
    go handleMessages(conn, airport)

    // Gestionar aterrizajes
    go manageRunways(airport)

    // Monitorear finalización de la simulación
    go monitorCompletion(airport)

    // Mantener la goroutine principal en ejecución hasta que la simulación termine
    wg.Add(1)
    <-airport.done
    wg.Done()
    wg.Wait()
}

func generateInitialAirplanes(airport *Airport, numA, numB, numC int) {
    id := 1
    planes := make([]Plane, 0, numA+numB+numC)

    // Generar aviones de cada categoría
    for i := 0; i < numA; i++ {
        planes = append(planes, Plane{
            id:       id,
            category: "A",
            numPass:  assignPassengers("A"),
            arrival:  time.Now(),
        })
        id++
    }
    for i := 0; i < numB; i++ {
        planes = append(planes, Plane{
            id:       id,
            category: "B",
            numPass:  assignPassengers("B"),
            arrival:  time.Now(),
        })
        id++
    }
    for i := 0; i < numC; i++ {
        planes = append(planes, Plane{
            id:       id,
            category: "C",
            numPass:  assignPassengers("C"),
            arrival:  time.Now(),
        })
        id++
    }

    // Mezclar aleatoriamente la lista de aviones
    rand.Shuffle(len(planes), func(i, j int) {
        planes[i], planes[j] = planes[j], planes[i]
    })

    // Encolar los aviones
    for _, plane := range planes {
        if airport.enqueuePlane(plane) {
            // Plane encolado exitosamente
        }
    }
}

func (airport *Airport) enqueuePlane(plane Plane) bool {
    airport.mu.Lock()
    defer airport.mu.Unlock()

    if len(airport.queue) >= airport.maxQueueCap {
        fmt.Printf("Cola para Categoría %s llena. Avión %d rechazado.\n", plane.category, plane.id)
        return false
    }

    airport.queue = append(airport.queue, plane)
    fmt.Printf("Avión %d de Categoría %s en cola para aterrizar.\n", plane.id, plane.category)
    return true
}

func handleMessages(conn net.Conn, airport *Airport) {
    scanner := bufio.NewScanner(conn)
    for scanner.Scan() {
        msg := strings.TrimSpace(scanner.Text())
        processMessage(msg, airport)
    }
    if err := scanner.Err(); err != nil {
        fmt.Println("Connection error:", err)
    }
    // Señalar cierre si la conexión se cierra
    airport.mu.Lock()
    airport.state = 9
    airport.priority = ""
    airport.mu.Unlock()
}

func processMessage(msg string, airport *Airport) {
    code, err := strconv.Atoi(msg)
    if err != nil {
        logger.Println("Invalid message:", msg)
        return
    }

    airport.mu.Lock()
    defer airport.mu.Unlock()

    switch code {
    case 0:
        airport.state = 0
        airport.priority = ""
        fmt.Println("")
        fmt.Println("Aeropuerto Inactivo: No hay Aterrizajes")
    case 1:
        airport.state = 1
        airport.priority = "A"
        fmt.Println("")
        fmt.Println("Solo Categoría A puede aterrizar")
    case 2:
        airport.state = 2
        airport.priority = "B"
        fmt.Println("")
        fmt.Println("Solo Categoría B puede aterrizar")
    case 3:
        airport.state = 3
        airport.priority = "C"
        fmt.Println("")
        fmt.Println("Solo Categoría C puede aterrizar")
    case 4:
        airport.state = 4
        airport.priority = "A"
        fmt.Println("")
        fmt.Println("Prioridad de aterrizaje para Categoría A")
    case 5:
        airport.state = 5
        airport.priority = "B"
        fmt.Println("")
        fmt.Println("Prioridad de aterrizaje para Categoría B")
    case 6:
        airport.state = 6
        airport.priority = "C"
        fmt.Println("")
        fmt.Println("Prioridad de aterrizaje para Categoría C")
    case 7, 8:
        // Mantener estado anterior
        fmt.Println("")
        fmt.Println("Estado no definido: Se mantiene el estado anterior")
    case 9:
        airport.state = 9
        airport.priority = ""
        fmt.Println("")
        fmt.Println("Aeropuerto Cerrado Temporalmente")
    default:
        fmt.Println("Código no reconocido")
    }
}

func manageRunways(airport *Airport) {
    for {
        airport.mu.Lock()
        // Si el aeropuerto está cerrado temporalmente o inactivo, espera
        if airport.state == 0 || airport.state == 9 {
            airport.mu.Unlock()
            time.Sleep(1 * time.Second)
            continue
        }

        // Ordenar la cola basada en categoría y llegada
        sort.Sort(airport.queue)

        var plane *Plane
        var planeIndex int

        switch airport.state {
        case 1, 2, 3:
            // Sólo permitir aterrizar aviones de la categoría específica
            desiredCategory := airport.priority
            for i, p := range airport.queue {
                if p.category == desiredCategory {
                    plane = &p
                    planeIndex = i
                    break
                }
            }
        case 4, 5, 6:
            // Prioridad a una categoría específica, pero permitir otros si no hay
            desiredCategory := airport.priority
            priorityFound := false
            for i, p := range airport.queue {
                if p.category == desiredCategory {
                    plane = &p
                    planeIndex = i
                    priorityFound = true
                    break
                }
            }
            if !priorityFound {
                // No hay aviones de la categoría prioritaria, permitir otros
                if len(airport.queue) > 0 {
                    plane = &airport.queue[0]
                    planeIndex = 0
                }
            }
        default:
            // En otros estados, permitir cualquier avión
            if len(airport.queue) > 0 {
                plane = &airport.queue[0]
                planeIndex = 0
            }
        }

        if plane != nil && len(airport.runways) > 0 {
            // Verificar si el avión puede aterrizar según el estado
            canLand := false
            switch airport.state {
            case 1, 2, 3:
                if plane.category == airport.priority {
                    canLand = true
                }
            case 4, 5, 6:
                if plane.category == airport.priority {
                    canLand = true
                } else {
                    // Permitir otros aviones solo si no hay aviones de la categoría prioritaria esperando
                    priorityWaiting := false
                    for _, p := range airport.queue {
                        if p.category == airport.priority {
                            priorityWaiting = true
                            break
                        }
                    }
                    if !priorityWaiting {
                        canLand = true
                    }
                }
            default:
                // Otros estados permiten cualquier aterrizaje
                canLand = true
            }

            if canLand {
                // Eliminar el avión de la cola
                airport.queue = append(airport.queue[:planeIndex], airport.queue[planeIndex+1:]...)
                airport.activeLandings++
                runway := <-airport.runways
                airport.mu.Unlock()

                go handleLanding(airport, *plane, runway)
            } else {
                // No se puede aterrizar este avión, pasar al siguiente
                airport.mu.Unlock()
                time.Sleep(500 * time.Millisecond)
            }
        } else {
            airport.mu.Unlock()
            time.Sleep(500 * time.Millisecond)
        }
    }
}

func handleLanding(airport *Airport, plane Plane, runway Runway) {
    fmt.Printf("Avión %d de Categoría %s está aterrizando en la Pista %d.\n", plane.id, plane.category, runway.id)
    landingTime := time.Duration(rand.Intn(3)+1) * time.Second
    time.Sleep(landingTime)
    fmt.Printf("Avión %d aterrizado y desplazándose a la puerta.\n", plane.id)

    // Asignar puerta
    select {
    case gate := <-airport.gates:
        fmt.Printf("Gate %d: Avión %d asignado a la puerta.\n", gate.id, plane.id)
        go handleGate(airport, plane, gate)
    default:
        fmt.Printf("No hay puertas disponibles para el Avión %d. Esperando...\n", plane.id)
        time.Sleep(1 * time.Second)
        airport.mu.Lock()
        airport.queue = append([]Plane{plane}, airport.queue...)
        airport.mu.Unlock()
    }

    // Liberar pista
    airport.runways <- runway

    airport.mu.Lock()
    airport.activeLandings--
    airport.landedPlanes++
    airport.mu.Unlock()
}

func handleGate(airport *Airport, plane Plane, gate Gate) {
    fmt.Printf("Gate %d: Avión %d iniciando desembarque.\n", gate.id, plane.id)
    disembarkTime := time.Duration(rand.Intn(3)+1) * time.Second
    time.Sleep(disembarkTime)
    fmt.Printf("Gate %d: Avión %d ha terminado el desembarque.\n", gate.id, plane.id)
    airport.mu.Lock()
    airport.activeGates--
    airport.mu.Unlock()
    airport.gates <- gate
}

func monitorCompletion(airport *Airport) {
    for {
        airport.mu.Lock()
        completed := airport.landedPlanes >= airport.totalPlanes &&
            len(airport.queue) == 0 &&
            airport.activeLandings == 0 &&
            airport.activeGates == 0
        airport.mu.Unlock()

        if completed && airport.state != 9 {
            airport.done <- true
            return
        }
        time.Sleep(1 * time.Second)
    }
}

func assignPassengers(category string) int {
    switch category {
    case "A":
        return rand.Intn(100) + 101
    case "B":
        return rand.Intn(51) + 50
    case "C":
        return rand.Intn(50)
    default:
        return 0
    }
}