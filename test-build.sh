#!/bin/bash
# Test which package causes build hang

echo "Testing individual packages..."

for pkg in storage store api benchmark config health query shutdown cache; do
    echo -n "Testing $pkg... "
    cd $pkg 2>/dev/null || continue
    timeout 3 go build . >/dev/null 2>&1 &
    pid=$!
    sleep 3
    if kill -0 $pid 2>/dev/null; then
        kill -9 $pid 2>/dev/null
        echo "HUNG ❌"
    else
        wait $pid
        if [ $? -eq 0 ]; then
            echo "OK ✓"
        else
            echo "ERROR"
        fi
    fi
    cd ..
done

echo ""
echo "Testing main cmd..."
timeout 5 go build ./cmd/mantisDB/ >/dev/null 2>&1
if [ $? -eq 124 ]; then
    echo "CMD HUNG ❌"
else
    echo "CMD OK ✓"
fi
