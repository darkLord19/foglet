/**
 * Dispatch callback on click outside of node
 */
export function clickOutside(node: HTMLElement, callback?: () => void) {
    const handleClick = (event: MouseEvent) => {
        if (
            node &&
            !node.contains(event.target as Node) &&
            !event.defaultPrevented
        ) {
            if (callback) {
                callback();
            } else {
                node.dispatchEvent(
                    new CustomEvent("click_outside", { detail: node }),
                );
            }
        }
    };

    document.addEventListener("click", handleClick, true);

    return {
        update(newCallback: () => void) {
            callback = newCallback;
        },
        destroy() {
            document.removeEventListener("click", handleClick, true);
        },
    };
}
