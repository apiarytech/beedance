(* The PLC program of the esp32hw firmware: a TON that restarts itself,
   so %QX0.0 pulses once every 1 s and two scans. On the device's clock the
   pulses come 1020 ms apart at 10 ms scans; embedded/esp32hw checks them
   against the host's clock. *)
PROGRAM Main
VAR
    t : TON;
    pulses : DINT;
    pulse AT %QX0.0 : BOOL;
END_VAR
t(IN := NOT t.Q, PT := T#1s);
IF t.Q THEN
    pulses := pulses + 1;
END_IF;
pulse := t.Q;
END_PROGRAM
