#!/bin/bash

# Function to build and run the program
run_program() {
    # Build the program
    go build -o stocd .

    # Check if the build was successful
    if [ $? -eq 0 ]; then
        echo "Build successful, running the program..."

        # Run the program in a loop, restart if it crashes
        while true; do
            # Run the program and capture the output
            ./stocd 2>&1 | tee stocd_output.log

            # Check if the program crashed (non-zero exit code)
            if [ $? -ne 0 ]; then
                echo "Program crashed, restarting..."

                # Save the last 50 lines of output to a crash log
                tail -n 50 stocd_output.log > stocd_crash_$(date +%Y%m%d%H%M%S).log
                sleep 2
            else
                echo "Program exited normally."
                break
            fi
        done
    else
        echo "Build failed, exiting..."
        exit 1
    fi
}

# Run the function
run_program
