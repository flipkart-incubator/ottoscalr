import csv
import sys
import os
import matplotlib.pyplot as plt

def plot_csv_data(csv_file, output_folder):
    timestamps = []
    values = []

    with open(csv_file, 'r', newline='') as csvfile:
        reader = csv.reader(csvfile)
        next(reader)  # Skip the header row
        for row in reader:
            timestamps.append(float(row[3]))  # Access the 4th column
            values.append(float(row[4]))      # Access the 5th column

    plt.plot(timestamps, values)
    plt.xlabel('Timestamp')
    plt.ylabel('CPU Usage')
    plt.title(f'CPU Usage for {os.path.basename(csv_file)}')

    # Save the plot in the output folder
    output_file = os.path.join(output_folder, os.path.splitext(os.path.basename(csv_file))[0] + ".png")
    plt.savefig(output_file, dpi=1500)
    plt.clf()


if len(sys.argv) != 3:
    print("Usage: python3 scriptname.py input_folder output_folder")
else:
    input_folder = sys.argv[1]
    output_folder = sys.argv[2]

    # Create the output folder if it doesn't exist
    if not os.path.exists(output_folder):
        os.makedirs(output_folder)

    # Iterate over CSV files in the input folder and generate plots
    for file in os.listdir(input_folder):
        if file.endswith(".csv"):
            csv_path = os.path.join(input_folder, file)
            plot_csv_data(csv_path, output_folder)
